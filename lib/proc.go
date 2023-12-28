package lib

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type KernelVersion struct {
	Major int
	Minor int
	Dot   int
}

func NewKernelVersion() (*KernelVersion, error) {
	f, err := os.Open("/proc/version")
	if err != nil {
		return nil, err
	}

	defer f.Close()

	procVersion, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(procVersion), " ")
	if len(parts) < 3 {
		return nil, fmt.Errorf("Unable to split proc version: %s", procVersion)
	}

	if !(parts[0] == "Linux" && parts[1] == "version") {
		return nil, fmt.Errorf("Kernel (%s) is not supported, requires Linux", parts[0])
	}

	version := parts[2]

	parts = strings.Split(version, ".")
	if len(parts) < 3 {
		return nil, fmt.Errorf("Unable to split kernel version: %s", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("Unable to parse major version: %s", major)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("Unable to parse minor version: %s", minor)
	}

	dot, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("Unable to parse minor version: %s", dot)
	}

	return &KernelVersion{Major: major, Minor: minor, Dot: dot}, nil
}

func (k *KernelVersion) Ok(version KernelVersion) bool {
	if version.Major < k.Major {
		return true
	} else if version.Major == k.Major && version.Minor < k.Minor {
		return true
	} else if version.Major == k.Major && version.Minor == k.Minor && version.Dot <= k.Dot {
		return true
	}

	return false
}

type Cloneflags struct {
	AllowPTRACE        bool
	ClearTID           bool
	CloneTID           bool
	DisablePTRACE      bool
	JoinCGroup         bool
	ParentPidfd        bool
	ParentTID          bool
	PrivateCGroup      bool
	PrivateClock       bool
	PrivateIO          bool
	PrivateIPC         bool
	PrivateMounts      bool
	PrivateNetwork     bool
	PrivatePID         bool
	PrivateUsers       bool
	PrivateUTS         bool
	PrivateTLS         bool
	ProtectSignals     bool
	ResetSignals       bool
	SetVFORK           bool
	SetPPID            bool
	SetThread          bool
	SetSystemV         bool
	ShareFSInfo        bool
	ShareFiles         bool
	ShareVirtualMemory bool

	SysProcAttr *syscall.SysProcAttr

	Errors         []error
	kernelVersion  KernelVersion
	kernelVersions map[string]KernelVersion
	caps           map[string][]string
	invalid        map[string][]string
	flagsRequired  map[string][]string
	configRequired map[string][]string
}

func NewCloneflags() (*Cloneflags, error) {

	kernelVersion, err := NewKernelVersion()
	if err != nil {
		return nil, err
	}

	return &Cloneflags{
		kernelVersion: *kernelVersion,
		kernelVersions: map[string]KernelVersion{
			"CHILD_CLEARTID": KernelVersion{2, 5, 49},
			"CHILD_SETTID":   KernelVersion{2, 5, 49},
			"CLEAR_SIGHAND":  KernelVersion{5, 5, 0},
			"FILES":          KernelVersion{2, 0, 0},
			"FS":             KernelVersion{2, 0, 0},

			"INTO_CGROUP": KernelVersion{5, 7, 0},
			"IO":          KernelVersion{2, 6, 25},
			"NEWCGROUP":   KernelVersion{4, 6, 0},
			"NEWIPC":      KernelVersion{2, 6, 19},
			"NEWNET":      KernelVersion{2, 6, 24},
			"NEWNS":       KernelVersion{2, 4, 19},
			"NEWPID":      KernelVersion{2, 6, 24},
			"NEWUSER":     KernelVersion{2, 6, 23},
			// Before Linux 3.8, use of CLONE_NEWUSER required that the
			// caller have three capabilities: CAP_SYS_ADMIN, CAP_SETUID,
			// and CAP_SETGID.  Starting with Linux 3.8, no privileges
			// are needed to create a user namespace.
			"NEWUSER_NOCAPS": KernelVersion{3, 8, 0},
			"NEWUTS":         KernelVersion{2, 6, 19},
			// The CLONE_PARENT flag can't be used in clone calls by the
			// global init process (PID 1 in the initial PID namespace)
			// and init processes in other PID namespaces.  This
			// restriction prevents the creation of multi-rooted process
			// trees as well as the creation of unreapable zombies in the
			// initial PID namespace.
			"PARENT":        KernelVersion{2, 3, 12},
			"PARENT_SETTID": KernelVersion{2, 5, 49},
			// "PID": [2]*KernelVersion{KernelVersion{2, 0 ,0}, KernelVersion{2, 5, 15}}
			"PIDFD":   KernelVersion{5, 2, 0},
			"PTRACE":  KernelVersion{2, 2, 0},
			"SETTLS":  KernelVersion{2, 5, 32},
			"SIGHAND": KernelVersion{2, 0, 0},
			// "STOPPED": [2]*KernelVersion{KernelVersion{2, 6, 0}, KernelVersion{2, 6, 38}}
			"SYSVMEM":  KernelVersion{2, 5, 10},
			"THREAD":   KernelVersion{2, 4, 0},
			"UNTRACED": KernelVersion{2, 5, 46},
			"VFORK":    KernelVersion{2, 2, 0},
			"VM":       KernelVersion{2, 0, 0},
		},
		caps: map[string][]string{
			"NEWCGROUP": []string{"SYS_ADMIN"},
			"NEWIPC":    []string{"SYS_ADMIN"},
			"NEWNS":     []string{"SYS_ADMIN"},
			"NEWPID":    []string{"SYS_ADMIN"},
			"NEWUSER":   []string{"SYS_ADMIN", "SETUID", "SETGID"},
			"NEWUTS":    []string{"SYS_ADMIN"},
		},
		invalid: map[string][]string{
			"CLEAR_SIGHAND": []string{"SIGHAND"},
			"NEWIPC":        []string{"SYSVSEM"},
			// It is not permitted to specify both
			//  CLONE_NEWNS and CLONE_FS in the same clone call.
			"NEWNS": []string{"FS"},
			// This flag can't be specified in conjunction
			//  with CLONE_THREAD or CLONE_PARENT.
			"NEWPID": []string{"THREAD", "PARENT"},
			//  This flag can't be specified in conjunction with
			//  CLONE_THREAD or CLONE_PARENT.  For security reasons,
			//  CLONE_NEWUSER cannot be specified in conjunction with
			//  CLONE_FS.
			"NEWUSER": []string{"THREAD", "PARENT", "FS"},
		},
		flagsRequired: map[string][]string{
			"CHILD_SETTID": []string{"VM"},
			"SIGHAND":      []string{"VM"},
			"THEARD":       []string{"SIGHAND", "VM"},
		},
		configRequired: map[string][]string{
			"IO": []string{"BLOCK"},
		},
		SysProcAttr: &syscall.SysProcAttr{},
	}, nil
}

func (c *Cloneflags) requiresKernelVersion(name string) bool {
	version := c.kernelVersions[name]
	if version.Ok(c.kernelVersion) {
		return true
	}
	c.Errors = append(c.Errors,
		fmt.Errorf("clone flag %s requires kernel %v, but running %v", version, c.kernelVersion))
	return false
}

func (c *Cloneflags) requiresCloneflagsSet(name string) bool {
	flags, ok := c.flagsRequired[name]
	if !ok {
		return true
	}

	ok = false
	for _, flag := range flags {
		switch flag {
		case "VM":
			if !c.ShareVirtualMemory {
				c.Errors = append(c.Errors,
					fmt.Errorf("clone flag %s requires ShareVirtualMemory", name))
			}
			ok = ok && c.ShareVirtualMemory
		case "SIGHAND":
			if !c.ProtectSignals {
				c.Errors = append(c.Errors,
					fmt.Errorf("clone flag %s requires ProtectSignals", name))
			}
			ok = ok && c.ProtectSignals
		}
	}

	return ok
}

func (c *Cloneflags) requiresCloneflagsUnset(name string) bool {
	flags, ok := c.invalid[name]
	if !ok {
		return true
	}

	ok = true
	for _, flag := range flags {
		switch flag {
		case "FS":
			if c.ShareFSInfo {
				c.Errors = append(c.Errors,
					fmt.Errorf("clone flag %s conflicts ShareFSInfo", name))
			}
			ok = ok && c.ShareFSInfo
		case "SYSVSEM":
			if c.SetSystemV {
				c.Errors = append(c.Errors,
					fmt.Errorf("clone flag %s requires SetSystemV", name))
			}
			ok = ok && c.SetSystemV
		case "THREAD":
			if c.SetThread {
				c.Errors = append(c.Errors,
					fmt.Errorf("clone flag %s conflicts SetThread", name))
			}
			ok = ok && c.SetThread
		case "PARENT":
			if c.SetPPID {
				c.Errors = append(c.Errors,
					fmt.Errorf("clone flag %s conflicts SetPPID", name))
			}
			ok = ok && c.SetPPID
		case "SIGHAND":
			if c.ProtectSignals {
				c.Errors = append(c.Errors,
					fmt.Errorf("clone flag %s conflicts ProtectSignals", name))
			}
			ok = ok && c.ProtectSignals
		}
	}

	return !ok
}

func (c *Cloneflags) requires(name string) bool {
	return c.requiresKernelVersion(name) &&
		c.requiresCloneflagsSet(name) &&
		c.requiresCloneflagsUnset(name)
}

func (c *Cloneflags) Set() *syscall.SysProcAttr {
	if c.ShareVirtualMemory && c.requires("VM") {
		// CLONE_VM = 0x00000100 // set if VM shared between processes
		c.SysProcAttr.Cloneflags |= syscall.CLONE_VM
	}

	if c.ProtectSignals {
		// CLONE_SIGHAND = 0x00000800 // set if signal handlers and blocked signals shared
		c.SysProcAttr.Cloneflags |= syscall.CLONE_SIGHAND
	}

	if c.ShareFSInfo && c.requires("FS") {
		// CLONE_FS = 0x00000200 // set if fs info shared between processes
		c.SysProcAttr.Cloneflags |= syscall.CLONE_FS
	}
	if c.ShareFiles && c.requires("FILES") {
		// CLONE_FILES = 0x00000400 // set if open files shared between processes
		c.SysProcAttr.Cloneflags |= syscall.CLONE_FILES
	}

	if c.AllowPTRACE && c.requires("PTRACE") {
		// CLONE_PTRACE = 0x00002000 // set if we want to let tracing continue on the child too
		c.SysProcAttr.Cloneflags |= syscall.CLONE_PTRACE
	}

	if c.SetVFORK && c.requires("VFORK") {
		// CLONE_VFORK = 0x00004000 // set if the parent wants the child to wake it up on mm_release
		c.SysProcAttr.Cloneflags |= syscall.CLONE_VFORK
	}

	if c.SetPPID && c.requires("PARENT") {
		// CLONE_PARENT = 0x00008000 // set if we want to have the same parent as the cloner
		c.SysProcAttr.Cloneflags |= syscall.CLONE_PARENT
	}

	if c.SetThread && c.requires("THREAD") {
		// CLONE_THREAD = 0x00010000 // Same thread group?
		c.SysProcAttr.Cloneflags |= syscall.CLONE_THREAD
	}

	if c.PrivateMounts && c.requires("NEWNS") {
		// CLONE_NEWNS = 0x00020000 // New mount namespace group
		c.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNS
	}

	if c.SetSystemV && c.requires("SYSVSEM") {
		// CLONE_SYSVSEM = 0x00040000 // share system V SEM_UNDO semantics
		c.SysProcAttr.Cloneflags |= syscall.CLONE_SYSVSEM
	}

	if c.ParentPidfd && c.requires("PIDFD") {
		// CLONE_PIDFD = 0x00001000 // set if a pidfd should be placed in parent
		c.SysProcAttr.Cloneflags |= syscall.CLONE_PIDFD
	}

	if c.ParentTID {
		// CLONE_PARENT_SETTID = 0x00100000 // set the TID in the parent
		c.SysProcAttr.Cloneflags |= syscall.CLONE_PARENT_SETTID
	}

	if c.ClearTID {
		// CLONE_CHILD_CLEARTID = 0x00200000 // clear the TID in the child
		c.SysProcAttr.Cloneflags |= syscall.CLONE_CHILD_CLEARTID
	}

	// CLONE_DETACHED = 0x00400000 // Unused, ignored

	if c.DisablePTRACE {
		// CLONE_UNTRACED = 0x00800000 // set if the tracing process can't force CLONE_PTRACE on this clone
		c.SysProcAttr.Cloneflags |= syscall.CLONE_UNTRACED
	}

	if c.CloneTID {
		// CLONE_CHILD_SETTID = 0x01000000 // set the TID in the child
		c.SysProcAttr.Cloneflags |= syscall.CLONE_CHILD_SETTID
	}

	if c.PrivateTLS {
		// CLONE_SETTLS = 0x00080000 // create a new TLS for the child
		c.SysProcAttr.Cloneflags |= syscall.CLONE_SETTLS
	}

	if c.PrivateCGroup {
		// CLONE_NEWCGROUP = 0x02000000 // New cgroup namespace
		c.SysProcAttr.Cloneflags |= syscall.CLONE_NEWCGROUP
	}

	if c.PrivateUTS {
		// CLONE_NEWUTS = 0x04000000 // New utsname namespace
		c.SysProcAttr.Cloneflags |= syscall.CLONE_NEWUTS
	}

	if c.PrivateIPC {
		// CLONE_NEWIPC = 0x08000000 // New ipc namespace
		c.SysProcAttr.Cloneflags |= syscall.CLONE_NEWIPC
	}

	if c.PrivateUsers {
		// CLONE_NEWUSER = 0x10000000 // New user namespace
		c.SysProcAttr.Cloneflags |= syscall.CLONE_NEWUSER
	}

	if c.PrivatePID {
		// CLONE_NEWPID = 0x20000000 // New pid namespace
		c.SysProcAttr.Cloneflags |= syscall.CLONE_NEWPID
	}

	if c.PrivateNetwork {
		// CLONE_NEWNET = 0x40000000 // New network namespace
		c.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}

	if c.PrivateIO {
		// CLONE_IO = 0x80000000 // Clone io context
		c.SysProcAttr.Cloneflags |= syscall.CLONE_IO
	}

	if c.ResetSignals {
		// CLONE_CLEAR_SIGHAND = 0x100000000 // Clear any signal handler and reset to SIG_DFL.
		c.SysProcAttr.Cloneflags |= syscall.CLONE_CLEAR_SIGHAND
	}

	if c.JoinCGroup {
		// CLONE_INTO_CGROUP = 0x200000000 // Clone into a specific cgroup given the right permissions.
		c.SysProcAttr.Cloneflags |= syscall.CLONE_INTO_CGROUP
	}

	if c.PrivateClock {
		// CLONE_NEWTIME = 0x00000080 // New time namespace
		c.SysProcAttr.Cloneflags |= syscall.CLONE_NEWTIME
	}

	return c.SysProcAttr
}

type Proc struct {
	Ctx    context.Context
	Chroot string

	Cloneflags *Cloneflags
	Uid        int
	Gid        int
}

func (p *Proc) listenerToFile(l net.Listener) (*os.File, error) {
	t, ok := l.(*net.TCPListener)
	if !ok {
		return nil, fmt.Errorf("Could not convert listener to a TCP Listener: %v", l)
	}

	return t.File()
}

func (p *Proc) ForkListener(l net.Listener) error {
	f, err := p.listenerToFile(l)
	if err != nil {
		return err
	}

	c := exec.CommandContext(p.Ctx, os.Args[0], "--listen.addr=FD:3")
	c.Env = os.Environ()
	c.Env = append(c.Env, "LISTEN_FDS=1", fmt.Sprintf("LISTEN_PID=%d", os.Getpid()))
	c.SysProcAttr = p.Cloneflags.Set()
	c.ExtraFiles = []*os.File{f}

	return c.Run()
}

func (p *Proc) ForkListenerPipe(l1 net.Listener, dial func(net.Conn)) error {
	f1, err := p.listenerToFile(l1)
	if err != nil {
		return err
	}

	pipe := &Pipe{}
	conns, err := pipe.Unixpair()
	if err != nil {
		return err
	}

	c := exec.CommandContext(p.Ctx, os.Args[0], "--listen.addr=FD:3", "--client.addr=FD:4")
	c.Env = []string{}
	c.Env = append(c.Env, "FIX_LISTEN_PID=1", "LISTEN_FDS=1", "LISTEN_FDNAMES=fd0", fmt.Sprintf("LISTEN_PID=%d", os.Getpid()))
	c.SysProcAttr = p.Cloneflags.Set()
	if p.Uid > 0 || p.Gid > 0 {
		c.SysProcAttr.Credential = &syscall.Credential{}
	}
	if p.Uid > 0 {
		c.SysProcAttr.Credential.Uid = uint32(p.Uid)
	}
	if p.Gid > 0 {
		c.SysProcAttr.Credential.Gid = uint32(p.Gid)
	}
	c.ExtraFiles = []*os.File{f1, pipe.Files[1]}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	defer conns[0].Close()
	if err := c.Start(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("unable to start process: %v, %s, %s", err, err.Stderr, c.Environ())
		}
		return fmt.Errorf("unable to start process: %v", err)
	}

	uc, ok := conns[0].(*net.UnixConn)
	if !ok {
		return fmt.Errorf("unable to convert conn to unixconn")
	}
	buf := make([]byte, 32)
	oob := make([]byte, 32)

	for {
		_, oobn, _, _, err := uc.ReadMsgUnix(buf, oob)
		if err != nil {
			fmt.Printf("ForkListener: err: readmsgunix: %v\n", err)
			continue
		}

		cmsgs, err := unix.ParseSocketControlMessage(oob[:oobn])
		if err != nil {
			fmt.Printf("ForkListener: err: parsesocketcontrolmessage: %v\n", err)
			continue
		}
		if len(cmsgs) != 1 {
			fmt.Printf("ForkListener: err: len(cmsgs) != 1 (%d)", len(cmsgs))
			continue
		}

		fds, err := unix.ParseUnixRights(&cmsgs[0])
		if err != nil {
			fmt.Printf("ForkListener: err: parseunixrights: %v\n", err)
			continue
		}

		f := os.NewFile(uintptr(fds[0]), "conn")

		fc, err := net.FileConn(f)
		if err != nil {
			fmt.Printf("ForkListener: err: fileconn: %v\n", err)
			continue
		}

		go dial(fc)
	}

	return c.Wait()
}

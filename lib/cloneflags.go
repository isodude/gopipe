package lib

import (
	"fmt"
	"syscall"
)

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
			"CHILD_CLEARTID": {2, 5, 49},
			"CHILD_SETTID":   {2, 5, 49},
			"CLEAR_SIGHAND":  {5, 5, 0},
			"FILES":          {2, 0, 0},
			"FS":             {2, 0, 0},

			"INTO_CGROUP": {5, 7, 0},
			"IO":          {2, 6, 25},
			"NEWCGROUP":   {4, 6, 0},
			"NEWIPC":      {2, 6, 19},
			"NEWNET":      {2, 6, 24},
			"NEWNS":       {2, 4, 19},
			"NEWPID":      {2, 6, 24},
			"NEWUSER":     {2, 6, 23},
			// Before Linux 3.8, use of CLONE_NEWUSER required that the
			// caller have three capabilities: CAP_SYS_ADMIN, CAP_SETUID,
			// and CAP_SETGID.  Starting with Linux 3.8, no privileges
			// are needed to create a user namespace.
			"NEWUSER_NOCAPS": {3, 8, 0},
			"NEWUTS":         {2, 6, 19},
			// The CLONE_PARENT flag can't be used in clone calls by the
			// global init process (PID 1 in the initial PID namespace)
			// and init processes in other PID namespaces.  This
			// restriction prevents the creation of multi-rooted process
			// trees as well as the creation of unreapable zombies in the
			// initial PID namespace.
			"PARENT":        {2, 3, 12},
			"PARENT_SETTID": {2, 5, 49},
			// "PID": [2]*KernelVersion{KernelVersion{2, 0 ,0}, {2, 5, 15}}
			"PIDFD":   {5, 2, 0},
			"PTRACE":  {2, 2, 0},
			"SETTLS":  {2, 5, 32},
			"SIGHAND": {2, 0, 0},
			// "STOPPED": [2]*KernelVersion{KernelVersion{2, 6, 0}, {2, 6, 38}}
			"SYSVMEM":  {2, 5, 10},
			"THREAD":   {2, 4, 0},
			"UNTRACED": {2, 5, 46},
			"VFORK":    {2, 2, 0},
			"VM":       {2, 0, 0},
		},
		caps: map[string][]string{
			"NEWCGROUP": {"SYS_ADMIN"},
			"NEWIPC":    {"SYS_ADMIN"},
			"NEWNS":     {"SYS_ADMIN"},
			"NEWPID":    {"SYS_ADMIN"},
			"NEWUSER":   {"SYS_ADMIN", "SETUID", "SETGID"},
			"NEWUTS":    {"SYS_ADMIN"},
		},
		invalid: map[string][]string{
			"CLEAR_SIGHAND": {"SIGHAND"},
			"NEWIPC":        {"SYSVSEM"},
			// It is not permitted to specify both
			//  CLONE_NEWNS and CLONE_FS in the same clone call.
			"NEWNS": {"FS"},
			// This flag can't be specified in conjunction
			//  with CLONE_THREAD or CLONE_PARENT.
			"NEWPID": {"THREAD", "PARENT"},
			//  This flag can't be specified in conjunction with
			//  CLONE_THREAD or CLONE_PARENT.  For security reasons,
			//  CLONE_NEWUSER cannot be specified in conjunction with
			//  CLONE_FS.
			"NEWUSER": {"THREAD", "PARENT", "FS"},
		},
		flagsRequired: map[string][]string{
			"CHILD_SETTID": {"VM"},
			"SIGHAND":      {"VM"},
			"THEARD":       {"SIGHAND", "VM"},
		},
		configRequired: map[string][]string{
			"IO": {"BLOCK"},
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
		fmt.Errorf("clone flag %s requires kernel %v, but running %v", name, version, c.kernelVersion))
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

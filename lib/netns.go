package lib

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/vishvananda/netns"
)

type NetworkNamespace struct {
	DockerName  string `long:"docker-name" description:"A docker identifier"`
	NetName     string `long:"net-name" description:"A iproute2 netns name"`
	Path        string `long:"path" description:"A netns path"`
	SystemdUnit string `long:"systemd-unit" description:"A systemd unit name"`
	PID         int    `long:"pid" description:"Process ID of a running process"`
	TID         int    `long:"tid" description:"Thread ID of a running thread inside a process"`
	Disable     bool   `long:"disable" description:"Do not try to use namespaces"`

	Protocol         string
	MainPID          int
	Ctx              context.Context
	previousNsHandle netns.NsHandle
	nsHandle         netns.NsHandle
	switched         bool
	lookedup         bool
	armed            bool
	Debug            bool `long:"debug"`
}

func (n *NetworkNamespace) IsSet() bool {
	if n.DockerName != "" || n.NetName != "" || n.Path != "" || n.SystemdUnit != "" || n.PID > 0 || n.TID > 0 {
		return true
	}
	return false
}

func (n *NetworkNamespace) Dialer(sourceIP string, timeout time.Duration) (*net.Dialer, error) {
	if sourceIP == "" {
		sourceIP = "[::]"
	}

	ip, err := net.ResolveTCPAddr(n.Protocol, fmt.Sprintf("%s:0", sourceIP))
	if err != nil {
		return nil, err
	}

	return &net.Dialer{
		LocalAddr: ip,
		Timeout:   timeout,
		Control: func(network, address string, c syscall.RawConn) error {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			if err, _ := n.Enter(); err != nil {
				return err
			}
			return nil
		},
	}, nil
}

func (n *NetworkNamespace) Close() {
	if n.previousNsHandle.IsOpen() {
		n.previousNsHandle.Close()
	}
	if n.nsHandle.IsOpen() {
		n.nsHandle.Close()
	}
}

func (n *NetworkNamespace) isOpen() error {
	if !n.nsHandle.IsOpen() {
		return fmt.Errorf("net fd closed for some reason")
	}

	return nil
}

func (n *NetworkNamespace) Enter() (err error, ok bool) {
	if n.Disable {
		return nil, false
	}

	if err = n.isOpen(); err != nil {
		err = fmt.Errorf("isOpen: %s", err)
		return
	}

	if !n.lookedup {
		if err, ok = n.refreshNetNSID(); err != nil {
			err = fmt.Errorf("refreshNetNSID: %s", err)
			return
		}
	}

	if !n.previousNsHandle.Equal(n.nsHandle) {
		if err = netns.Set(n.nsHandle); err != nil {
			err = fmt.Errorf("netns: Set: %s", err)
			return
		}
		n.switched = true
	}

	return
}

func (n *NetworkNamespace) Exit() (err error) {
	if !n.switched {
		return nil
	}

	if err = n.isOpen(); err != nil {
		return err
	}

	if err = netns.Set(n.previousNsHandle); err != nil {
		return fmt.Errorf("failed to switch back to ns: %v", err)
	}

	n.switched = false

	return nil
}

func (n *NetworkNamespace) SetCurrent() error {
	h, err := netns.Get()
	if err != nil {
		return err
	}

	n.previousNsHandle = h
	return nil
}

func (n *NetworkNamespace) refreshNetNSID() (error, bool) {

	defer func() {
		n.lookedup = true
	}()

	var errors []string

	if n.PID < 1 && n.SystemdUnit != "" {
		pid, err := n.getSystemdUnitMainPID(n.SystemdUnit)
		if err == nil {
			var h netns.NsHandle
			h, err = netns.GetFromPid(int(pid))
			if err == nil {
				n.nsHandle = h
				n.armed = true
				return nil, true
			}
		}
		errors = append(errors, fmt.Sprintf("systemd.unit: %s", err))
	}

	// Handle both Systemd and PID directly
	if n.PID > 0 {
		h, err := netns.GetFromPid(n.PID)
		if err == nil {
			n.nsHandle = h
			n.armed = true
			return nil, true
		}
		errors = append(errors, fmt.Sprintf("pid: %s", err))
	}

	if n.PID > 0 && n.TID > 0 {
		h, err := netns.GetFromThread(n.PID, n.TID)
		if err == nil {
			n.nsHandle = h
			n.armed = true
			return nil, true
		}
		errors = append(errors, fmt.Sprintf("tid: %s", err))
	}

	if n.NetName != "" {
		h, err := netns.GetFromName(n.NetName)
		if err == nil {
			n.nsHandle = h
			n.armed = true
			return nil, true
		}
		errors = append(errors, fmt.Sprintf("net-name: %s", err))
	}

	if n.DockerName != "" {
		h, err := netns.GetFromDocker(n.DockerName)
		if err == nil {
			n.nsHandle = h
			n.armed = true
			return nil, true
		}
		errors = append(errors, fmt.Sprintf("docker-name: %s", err))
	}

	if n.Path != "" {
		h, err := netns.GetFromPath(n.Path)
		if err == nil {
			n.nsHandle = h
			n.armed = true
			return nil, true
		}
		errors = append(errors, fmt.Sprintf("path: %s", err))
	}

	if !n.armed {
		h, err := netns.Get()
		if err == nil {
			n.nsHandle, n.previousNsHandle = n.previousNsHandle, h
			n.armed = true
			return nil, true
		}
		errors = append(errors, fmt.Sprintf("path: %s", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, ", ")), false
	}

	return nil, false
}

func (n *NetworkNamespace) getSystemdUnitMainPID(unit string) (uint32, error) {
	conn, err := dbus.NewWithContext(n.Ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	props, err := conn.GetAllPropertiesContext(n.Ctx, unit)
	if err != nil {
		return 0, err
	}

	var pid uint32
	if value, ok := props["ExecMainPID"]; !ok {
		return 0, fmt.Errorf("could not find %s", "ExecMainPID")
	} else if pid, ok = value.(uint32); !ok {
		return 0, fmt.Errorf("value was not property: %v", value)
	}

	return pid, nil
}

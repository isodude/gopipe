package lib

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
		return nil, fmt.Errorf("unable to split proc version: %s", procVersion)
	}

	if !(parts[0] == "Linux" && parts[1] == "version") {
		return nil, fmt.Errorf("kernel (%s) is not supported, requires Linux", parts[0])
	}

	version := parts[2]

	parts = strings.Split(version, ".")
	if len(parts) < 3 {
		return nil, fmt.Errorf("unable to split kernel version: %s", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("unable to parse major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("unable to parse minor version: %s", parts[1])
	}

	dot, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("unable to parse minor version: %s", parts[1])
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

package lib

import (
	"fmt"
	"os"
	"testing"
)

// TestE2EBin isn't a real test.
func TestE2EBin(t *testing.T) {
	if os.Getenv("CMD_TEST_E2E") != "1" {
		t.SkipNow()
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(3)
	}

	MainFunc(args)
}

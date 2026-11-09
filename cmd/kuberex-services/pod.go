package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"sarak.sh/mikluko/kuberex/internal/account"
)

// pod is the host of a kuberex pod, whose login server keeps each account's
// runtime directory in runtimeRoot/accounts/<account>.
type pod struct{ runtimeRoot string }

func (p pod) serverReady(name string) bool {
	c, err := net.Dial("unix", filepath.Join(account.RuntimeDir(p.runtimeRoot, name), "server.sock"))
	if err != nil {
		return false
	}
	return c.Close() == nil
}

// startServer starts the server in a session of its own, so it outlives this
// process and is reaped by PID 1.
func (p pod) startServer(name string) error {
	cmd := exec.Command("rex", "login-server", "session",
		"--account", name, "--runtime-root", p.runtimeRoot,
		"--", "server", "run", "--exit-empty")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

func (p pod) rex(name string, args ...string) ([]byte, error) {
	cmd, err := account.Rex(p.runtimeRoot, name, args...)
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rex %v: %w: %s", args, err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}

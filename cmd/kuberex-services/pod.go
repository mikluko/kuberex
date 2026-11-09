package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

// pod is the host of a kuberex pod, whose login server keeps each account's
// runtime directory in runtimeRoot/accounts/<account>.
type pod struct{ runtimeRoot string }

func (p pod) runtimeDir(account string) string {
	return filepath.Join(p.runtimeRoot, "accounts", account)
}

func (p pod) serverReady(account string) bool {
	c, err := net.Dial("unix", filepath.Join(p.runtimeDir(account), "server.sock"))
	if err != nil {
		return false
	}
	return c.Close() == nil
}

// startServer starts the server in a session of its own, so it outlives this
// process and is reaped by PID 1.
func (p pod) startServer(account string) error {
	cmd := exec.Command("rex", "login-server", "session",
		"--account", account, "--runtime-root", p.runtimeRoot,
		"--", "server", "run", "--exit-empty")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

func (p pod) rex(account string, args ...string) ([]byte, error) {
	u, err := user.Lookup(account)
	if err != nil {
		return nil, err
	}
	cred, err := credential(u)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("rex", append([]string{"--autostart=false"}, args...)...)
	cmd.Dir = u.HomeDir
	cmd.Env = []string{
		"HOME=" + u.HomeDir,
		"USER=" + account,
		"LOGNAME=" + account,
		"PATH=" + os.Getenv("PATH"),
		"REX_RUNTIME_DIR=" + p.runtimeDir(account),
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rex %v: %w: %s", args, err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}

func credential(u *user.User) (*syscall.Credential, error) {
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	gids, err := u.GroupIds()
	if err != nil {
		return nil, err
	}
	groups := make([]uint32, 0, len(gids))
	for _, g := range gids {
		n, err := strconv.ParseUint(g, 10, 32)
		if err != nil {
			return nil, err
		}
		groups = append(groups, uint32(n))
	}
	return &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: groups}, nil
}

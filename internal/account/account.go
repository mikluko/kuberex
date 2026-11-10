// Package account starts an account's own Rex server below the login server's
// runtime root, and runs rex as that account against it.
package account

import (
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

// RuntimeDir returns the runtime directory the login server keeps for name
// below runtimeRoot, where name's server listens on server.sock.
func RuntimeDir(runtimeRoot, name string) string {
	return filepath.Join(runtimeRoot, "accounts", name)
}

// ServerSocket returns the socket name's server listens on below runtimeRoot.
func ServerSocket(runtimeRoot, name string) string {
	return filepath.Join(RuntimeDir(runtimeRoot, name), "server.sock")
}

// ServerReady reports whether name's server accepts connections.
func ServerReady(runtimeRoot, name string) bool {
	c, err := net.Dial("unix", ServerSocket(runtimeRoot, name))
	if err != nil {
		return false
	}
	return c.Close() == nil
}

// StartServer starts name's server the way the login server does, so the
// login server adopts it, and returns without waiting for it to accept
// connections. The server runs in a session of its own, outliving the caller
// to be reaped by PID 1. With exitEmpty it stops once no sessions or control
// connections remain.
func StartServer(runtimeRoot, name string, exitEmpty bool) error {
	args := []string{"login-server", "session", "--account", name, "--runtime-root", runtimeRoot, "--", "server", "run"}
	if exitEmpty {
		args = append(args, "--exit-empty")
	}
	cmd := exec.Command("rex", args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

// Rex returns a command running rex with args as name, against name's own
// server, with autostart off. The caller must run as root.
func Rex(runtimeRoot, name string, args ...string) (*exec.Cmd, error) {
	u, err := user.Lookup(name)
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
		"USER=" + name,
		"LOGNAME=" + name,
		"PATH=" + os.Getenv("PATH"),
		"REX_RUNTIME_DIR=" + RuntimeDir(runtimeRoot, name),
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred}
	return cmd, nil
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

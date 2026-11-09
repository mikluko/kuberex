// Package account runs rex as an account of the pod, against that account's
// own Rex server below the login server's runtime root.
package account

import (
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

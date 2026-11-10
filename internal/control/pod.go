package control

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"sarak.sh/mikluko/kuberex/internal/account"
)

// stopTimeout bounds how long one account's server may take to stop.
const stopTimeout = time.Minute

// pod is the host of a kuberex pod.
type pod struct {
	runtimeRoot    string
	loginServerPID int
	marker         string
}

func (p pod) markShutdown() error { return os.WriteFile(p.marker, nil, 0o644) }

func (p pod) servers() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(p.runtimeRoot, "accounts"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if account.ServerReady(p.runtimeRoot, e.Name()) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func (p pod) stopServer(name string) error {
	cmd, err := account.Rex(p.runtimeRoot, name, "server", "stop")
	if err != nil {
		return err
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%w: %s", err, bytes.TrimSpace(out.Bytes()))
		}
		return nil
	case <-time.After(stopTimeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("rex server stop did not finish within %s", stopTimeout)
	}
}

// sync has no error on Linux; elsewhere it returns one, which is ignored.
func (pod) sync() { syscall.Sync() } //nolint:errcheck

func (p pod) terminate() error { return syscall.Kill(p.loginServerPID, syscall.SIGTERM) }

func lookupUID(uid uint32) (string, error) {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

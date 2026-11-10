package services

import (
	"bytes"
	"fmt"

	"sarak.sh/mikluko/kuberex/internal/account"
)

// pod is the host of a kuberex pod, whose login server keeps each account's
// runtime directory in runtimeRoot/accounts/<account>.
type pod struct{ runtimeRoot string }

func (p pod) serverReady(name string) bool { return account.ServerReady(p.runtimeRoot, name) }

func (p pod) startServer(name string) error { return account.StartServer(p.runtimeRoot, name, true) }

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

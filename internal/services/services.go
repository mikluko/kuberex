// Package services starts the services a JSON file lists: for each, a Rex
// session labelled as the service, running its command in its account's own
// Rex server. "kuberexctl services" runs as root at pod start and exits once
// every service has started; it does not restart a command that exits.
//
// The account's server is started the way the login server starts one, with
// rex login-server session, so the login server adopts it rather than starting
// a second. A session that already carries the label is one the server
// restored from before the pod restarted, holding a bare shell: it is killed,
// and the service starts in a new session of the same label.
package services

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"time"
)

// service is one entry of the services file.
type service struct {
	Account string   `json:"account"`
	Label   string   `json:"label"`
	Cwd     string   `json:"cwd"`
	Command []string `json:"command"`
}

func parseServices(src []byte) ([]service, error) {
	var svcs []service
	if err := json.Unmarshal(src, &svcs); err != nil {
		return nil, err
	}
	for i, s := range svcs {
		if s.Account == "" || s.Label == "" || len(s.Command) == 0 {
			return nil, fmt.Errorf("service %d: account, label and command are required", i)
		}
	}
	return svcs, nil
}

// host is what starting a service needs from the pod.
type host interface {
	// serverReady reports whether account's Rex server accepts connections.
	serverReady(account string) bool
	// startServer starts account's Rex server and returns without waiting
	// for it to accept connections.
	startServer(account string) error
	// rex runs rex as account against account's own server and returns its
	// standard output.
	rex(account string, args ...string) ([]byte, error)
}

func ensureServer(h host, account string, timeout time.Duration) error {
	if h.serverReady(account) {
		return nil
	}
	if err := h.startServer(account); err != nil {
		return err
	}
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if h.serverReady(account) {
			return nil
		}
	}
	return fmt.Errorf("the Rex server of %s did not start within %s", account, timeout)
}

// sessionsLabelled returns the IDs of the sessions in rex ls --json output
// that carry label.
func sessionsLabelled(ls []byte, label string) ([]string, error) {
	var out struct {
		Sessions []struct {
			ID    string `json:"session_id"`
			Label string `json:"label"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(ls, &out); err != nil {
		return nil, fmt.Errorf("reading rex ls: %w", err)
	}
	var ids []string
	for _, s := range out.Sessions {
		if s.Label == label {
			ids = append(ids, s.ID)
		}
	}
	return ids, nil
}

// timeouts bounds the waits of starting a service.
type timeouts struct {
	// server is how long an account's Rex server may take to start.
	server time.Duration
	// kill is how long the sessions a service replaces may take to go.
	kill time.Duration
}

// killSessions kills the sessions ids of s's account and returns once they are
// gone. A kill that errors, as one of a session with many processes can when
// it outlasts rex's request timeout, is not taken as failure: the sessions
// are looked for until timeout, and only one still there fails.
func killSessions(h host, s service, ids []string, timeout time.Duration) error {
	if len(ids) == 0 {
		return nil
	}
	args := append([]string{"kill", "--timeout", timeout.String()}, ids...)
	_, killErr := h.rex(s.Account, args...)
	if killErr == nil {
		return nil
	}
	var left []string
	for deadline := time.Now().Add(timeout); ; time.Sleep(100 * time.Millisecond) {
		if ls, err := h.rex(s.Account, "ls", "--json"); err == nil {
			if left, err = sessionsLabelled(ls, s.Label); err == nil && !slices.ContainsFunc(left, func(id string) bool {
				return slices.Contains(ids, id)
			}) {
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			break
		}
	}
	return fmt.Errorf("sessions %v still there %s after killing them: %w", ids, timeout, killErr)
}

// startService replaces any session labelled as s with a new one running its
// command. The terminal stays open after the command exits, so its last
// output can be read.
func startService(h host, s service, t timeouts) error {
	if err := ensureServer(h, s.Account, t.server); err != nil {
		return err
	}
	ls, err := h.rex(s.Account, "ls", "--json")
	if err != nil {
		return err
	}
	ids, err := sessionsLabelled(ls, s.Label)
	if err != nil {
		return err
	}
	if err := killSessions(h, s, ids, t.kill); err != nil {
		return err
	}
	args := []string{"new", s.Label, "--keep-open"}
	if s.Cwd != "" {
		args = append(args, "--cwd", s.Cwd)
	}
	args = append(append(args, "--"), s.Command...)
	_, err = h.rex(s.Account, args...)
	return err
}

// Main runs "kuberexctl services" with args, the words after "services", and
// returns its exit code.
func Main(args []string) int {
	fs := flag.NewFlagSet("services", flag.ExitOnError)
	runtimeRoot := fs.String("runtime-root", "/run/rex", "the login server's runtime root")
	timeout := fs.Duration("timeout", 30*time.Second, "how long to wait for an account's Rex server to start")
	killTimeout := fs.Duration("kill-timeout", 2*time.Minute, "how long the sessions a service replaces may take to go")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: kuberexctl services [flags] SERVICES-FILE")
		return 2
	}
	src, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "kuberexctl services:", err)
		return 1
	}
	svcs, err := parseServices(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kuberexctl services: %s: %v\n", fs.Arg(0), err)
		return 1
	}
	h := pod{runtimeRoot: *runtimeRoot}
	var failed error
	for _, s := range svcs {
		if err := startService(h, s, timeouts{server: *timeout, kill: *killTimeout}); err != nil {
			fmt.Fprintf(os.Stderr, "kuberexctl services: %s of %s: %v\n", s.Label, s.Account, err)
			failed = errors.Join(failed, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "kuberexctl services: started %s as %s\n", s.Label, s.Account)
	}
	if failed != nil {
		return 1
	}
	return 0
}

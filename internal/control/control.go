// Package control lets an account shut the pod's server container down
// gracefully. Whether the container starts again is up to its restart policy.
//
// "kuberexctl serve" runs as root beside the login server and listens on a
// unix socket. "kuberexctl shutdown", run as an account the server allows,
// asks it to shut down. The server then marks the shutdown and sends SIGTERM
// to the login server. Once the login server has exited, the entrypoint runs
// "kuberexctl stop-servers", which stops every account's Rex server, so each
// records a clean shutdown and restores its sessions at the next start, and
// syncs the filesystems; a marked shutdown then exits with code 0.
package control

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultSocket = "/run/kuberex/control.sock"

// defaultMarker is the file "kuberexctl serve" creates when it accepts a
// shutdown, which the entrypoint reads.
const defaultMarker = "/run/kuberex/shutdown"

// ServeMain runs "kuberexctl serve" with args, the words after "serve". It
// returns only to exit the process with a failure.
func ServeMain(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	socket := fs.String("socket", defaultSocket, "unix socket to listen on")
	runtimeRoot := fs.String("runtime-root", "/run/rex", "the login server's runtime root")
	pid := fs.Int("login-server-pid", 0, "process ID of the login server")
	accounts := fs.String("accounts", "", "accounts allowed to shut down, separated by spaces or commas")
	marker := fs.String("marker", defaultMarker, "file created when a shutdown is accepted")
	_ = fs.Parse(args)
	log.SetPrefix("kuberexctl: ")
	log.SetFlags(0)
	if *pid <= 0 {
		log.Fatal("--login-server-pid is required")
	}
	s := newServer(parseAccounts(*accounts), lookupUID, pod{runtimeRoot: *runtimeRoot, loginServerPID: *pid, marker: *marker})
	log.Fatal(s.listen(*socket))
}

// parseAccounts splits a list of account names separated by spaces or commas.
func parseAccounts(s string) map[string]bool {
	m := map[string]bool{}
	for _, name := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		m[name] = true
	}
	return m
}

// ShutdownMain runs "kuberexctl shutdown" with args, the words after
// "shutdown", and returns its exit code.
func ShutdownMain(args []string) int {
	fs := flag.NewFlagSet("shutdown", flag.ExitOnError)
	socket := fs.String("socket", defaultSocket, "unix socket of kuberexctl serve")
	_ = fs.Parse(args)
	reply, err := request(*socket, "shutdown")
	if err != nil {
		fmt.Fprintf(os.Stderr, "kuberexctl: %v\n", err)
		return 1
	}
	if reply != "ok" {
		fmt.Fprintf(os.Stderr, "kuberexctl: %s\n", strings.TrimPrefix(reply, "error: "))
		return 1
	}
	fmt.Println("kuberexctl: shutting down the container")
	return 0
}

// request sends req to the server at socket and returns its one-line reply.
func request(socket, req string) (string, error) {
	c, err := net.Dial("unix", socket)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("shutdown is not enabled in this pod")
		}
		return "", err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := fmt.Fprintln(c, req); err != nil {
		return "", err
	}
	reply, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading the reply: %w", err)
	}
	return strings.TrimSpace(reply), nil
}

// requestHost is what accepting a shutdown needs from the pod.
type requestHost interface {
	// markShutdown records that a shutdown was requested, for the
	// entrypoint to exit 0 once the login server has exited.
	markShutdown() error
	// terminate sends SIGTERM to the login server.
	terminate() error
}

// serversHost is what stopping the account servers needs from the pod.
type serversHost interface {
	// servers returns the accounts whose Rex server is running.
	servers() ([]string, error)
	// stopServer stops account's Rex server and waits for it to exit.
	stopServer(account string) error
	// sync flushes every filesystem.
	sync()
}

type server struct {
	allowed map[string]bool
	lookup  func(uid uint32) (string, error)
	h       requestHost
	// grace is how long a shutdown waits after replying, so the requesting
	// process sees the reply before its own server stops.
	grace time.Duration

	mu           sync.Mutex
	shuttingDown bool
}

func newServer(allowed map[string]bool, lookup func(uint32) (string, error), h requestHost) *server {
	return &server{allowed: allowed, lookup: lookup, h: h, grace: time.Second}
}

// decide answers req from the process running as uid, and reports whether a
// shutdown is to begin. Only the first permitted request starts one.
func (s *server) decide(uid uint32, req string) (reply string, start bool) {
	name, err := s.lookup(uid)
	if err != nil || !s.allowed[name] {
		return fmt.Sprintf("error: uid %d is not allowed to shut the container down", uid), false
	}
	if req != "shutdown" {
		return fmt.Sprintf("error: unknown request %q", req), false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shuttingDown {
		return "error: the container is already shutting down", false
	}
	s.shuttingDown = true
	return "ok", true
}

// shutdown marks the shutdown and terminates the login server. The login
// server goes first so that no client can start an account server while the
// entrypoint stops them; a failed mark is logged and does not stop it.
func (s *server) shutdown() error {
	if err := s.h.markShutdown(); err != nil {
		log.Printf("marking the shutdown: %v", err)
	}
	return s.h.terminate()
}

// stopServers stops every account's server and syncs, and returns how many
// servers failed to stop. A failure does not hold up the other servers.
func stopServers(h serversHost) int {
	names, err := h.servers()
	if err != nil {
		log.Printf("listing account servers: %v", err)
	}
	failed := 0
	for _, name := range names {
		if err := h.stopServer(name); err != nil {
			log.Printf("stopping the Rex server of %s: %v", name, err)
			failed++
		} else {
			log.Printf("stopped the Rex server of %s", name)
		}
	}
	h.sync()
	return failed
}

// StopServersMain runs "kuberexctl stop-servers" with args, the words after
// "stop-servers", and returns its exit code: 1 when a server failed to stop.
func StopServersMain(args []string) int {
	fs := flag.NewFlagSet("stop-servers", flag.ExitOnError)
	runtimeRoot := fs.String("runtime-root", "/run/rex", "the login server's runtime root")
	_ = fs.Parse(args)
	log.SetPrefix("kuberexctl: ")
	log.SetFlags(0)
	if stopServers(pod{runtimeRoot: *runtimeRoot}) > 0 {
		return 1
	}
	return 0
}

// listen serves requests on socket until accepting fails, and returns why.
func (s *server) listen(socket string) error {
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	l, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	// Anyone may connect; decide authorizes by the peer's uid.
	if err := os.Chmod(socket, 0o666); err != nil {
		return err
	}
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		go s.serve(c)
	}
}

func (s *server) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	uid, err := peerUID(c)
	if err != nil {
		log.Printf("reading the peer's credentials: %v", err)
		return
	}
	req, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return
	}
	reply, start := s.decide(uid, strings.TrimSpace(req))
	_, _ = fmt.Fprintln(c, reply)
	if !start {
		log.Printf("refused %q from uid %d: %s", strings.TrimSpace(req), uid, reply)
		return
	}
	log.Printf("shutdown requested by uid %d", uid)
	go func() {
		time.Sleep(s.grace)
		if err := s.shutdown(); err != nil {
			log.Printf("terminating the login server: %v", err)
		}
	}()
}

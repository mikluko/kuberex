// Command kuberexctl lets an account shut the pod's server container down
// gracefully. Whether the container starts again is up to its restart policy.
//
// "kuberexctl serve" runs as root beside the login server and listens on a
// unix socket. "kuberexctl shutdown", run as an account the server allows,
// asks it to shut down. The server then stops every account's Rex server, so
// each records a clean shutdown and restores its sessions at the next start,
// syncs the filesystems and sends SIGTERM to the login server, whose exit
// ends the container with code 0.
package main

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

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "serve":
		serveMain(os.Args[2:])
	case "shutdown":
		os.Exit(shutdownMain(os.Args[2:]))
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: kuberexctl serve --login-server-pid PID --accounts NAMES | kuberexctl shutdown")
	os.Exit(2)
}

func serveMain(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	socket := fs.String("socket", defaultSocket, "unix socket to listen on")
	runtimeRoot := fs.String("runtime-root", "/run/rex", "the login server's runtime root")
	pid := fs.Int("login-server-pid", 0, "process ID of the login server")
	accounts := fs.String("accounts", "", "accounts allowed to shut down, separated by spaces or commas")
	_ = fs.Parse(args)
	log.SetPrefix("kuberexctl: ")
	log.SetFlags(0)
	if *pid <= 0 {
		log.Fatal("--login-server-pid is required")
	}
	s := newServer(parseAccounts(*accounts), lookupUID, pod{runtimeRoot: *runtimeRoot, loginServerPID: *pid})
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

func shutdownMain(args []string) int {
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

// host is what a shutdown needs from the pod.
type host interface {
	// servers returns the accounts whose Rex server is running.
	servers() ([]string, error)
	// stopServer stops account's Rex server and waits for it to exit.
	stopServer(account string) error
	// sync flushes every filesystem.
	sync()
	// terminate sends SIGTERM to the login server.
	terminate() error
}

type server struct {
	allowed map[string]bool
	lookup  func(uid uint32) (string, error)
	h       host
	// grace is how long a shutdown waits after replying, so the requesting
	// process sees the reply before its own server stops.
	grace time.Duration

	mu           sync.Mutex
	shuttingDown bool
}

func newServer(allowed map[string]bool, lookup func(uint32) (string, error), h host) *server {
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

// shutdown stops every account's server, syncs and terminates the login
// server. A server that fails to stop is logged and does not hold up the
// others.
func (s *server) shutdown() error {
	names, err := s.h.servers()
	if err != nil {
		log.Printf("listing account servers: %v", err)
	}
	for _, name := range names {
		if err := s.h.stopServer(name); err != nil {
			log.Printf("stopping the Rex server of %s: %v", name, err)
		} else {
			log.Printf("stopped the Rex server of %s", name)
		}
	}
	s.h.sync()
	return s.h.terminate()
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

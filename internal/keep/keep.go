// Package keep keeps the Rex servers of some accounts running without a
// session holding them. The login server starts an account's server with
// --exit-empty, so a server serving only processes outside its sessions, such
// as an account container's, would stop; "kuberexctl keep-servers" starts each
// of its accounts' servers without that flag, and starts it again whenever it
// is gone.
package keep

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sarak.sh/mikluko/kuberex/internal/account"
)

// host is what keeping servers needs from the pod.
type host interface {
	// serverReady reports whether account's Rex server accepts connections.
	serverReady(account string) bool
	// startServer starts account's Rex server, one that does not stop when
	// empty, and returns without waiting for it to accept connections.
	startServer(account string) error
}

// pass starts the server of each of accounts that does not accept
// connections, and waits up to timeout for it to. Once ctx is done it starts
// no more servers, but still waits for one it started.
func pass(ctx context.Context, h host, accounts []string, timeout, poll time.Duration, logf func(string, ...any)) error {
	var failed error
	for _, name := range accounts {
		if h.serverReady(name) {
			continue
		}
		if ctx.Err() != nil {
			return failed
		}
		if err := start(h, name, timeout, poll); err != nil {
			logf("the Rex server of %s: %v", name, err)
			failed = errors.Join(failed, err)
			continue
		}
		logf("started the Rex server of %s", name)
	}
	return failed
}

func start(h host, name string, timeout, poll time.Duration) error {
	if err := h.startServer(name); err != nil {
		return err
	}
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(poll) {
		if h.serverReady(name) {
			return nil
		}
	}
	return fmt.Errorf("did not start within %s", timeout)
}

// keep runs a pass every interval until ctx is done.
func keep(ctx context.Context, h host, accounts []string, interval, timeout time.Duration, logf func(string, ...any)) {
	for {
		_ = pass(ctx, h, accounts, timeout, 100*time.Millisecond, logf)
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

type pod struct{ runtimeRoot string }

func (p pod) serverReady(name string) bool { return account.ServerReady(p.runtimeRoot, name) }

func (p pod) startServer(name string) error { return account.StartServer(p.runtimeRoot, name, false) }

// Main runs "kuberexctl keep-servers" with args, the words after
// "keep-servers", and returns its exit code. It runs until SIGTERM or SIGINT
// and ignores SIGHUP, which dumb-init forwards to it with the login server's.
func Main(args []string) int {
	fs := flag.NewFlagSet("keep-servers", flag.ExitOnError)
	runtimeRoot := fs.String("runtime-root", "/run/rex", "the login server's runtime root")
	once := fs.Bool("once", false, "start the servers that are not running, wait for them, and exit")
	interval := fs.Duration("interval", 2*time.Second, "how often to look for a server that is gone")
	timeout := fs.Duration("timeout", 30*time.Second, "how long to wait for a server to start")
	_ = fs.Parse(args)
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: kuberexctl keep-servers [flags] ACCOUNT...")
		return 2
	}
	logf := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "kuberexctl keep-servers: "+format+"\n", a...)
	}
	h := pod{runtimeRoot: *runtimeRoot}
	if *once {
		if pass(context.Background(), h, fs.Args(), *timeout, 100*time.Millisecond, logf) != nil {
			return 1
		}
		return 0
	}
	signal.Ignore(syscall.SIGHUP)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	keep(ctx, h, fs.Args(), *interval, *timeout, logf)
	return 0
}

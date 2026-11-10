// Package runas runs a command as an account of the pod from a container
// beside the server container, an account container. "kuberexctl run" starts
// as root, waits for the server container to set the accounts up, gives the
// account the uid that owns its home, and execs the command as the account,
// with its Rex server's runtime directory and autostart off: a server it
// started would run in the account container, outside the login server's.
package runas

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sarak.sh/mikluko/kuberex/internal/account"
)

// DefaultReady is the file the entrypoint creates once it has set the
// accounts, their Secrets and their temporary directories up.
const DefaultReady = "/run/kuberex/ready"

// options is what "kuberexctl run" was asked to do.
type options struct {
	account     string
	homeRoot    string
	runtimeRoot string
	ready       string
	cwd         string
	waitServer  bool
	command     []string
}

// ident is an account's uid and gid.
type ident struct{ uid, gid uint32 }

// host is what running a command as an account needs from the container.
type host interface {
	exists(path string) bool
	// owner returns the uid and gid owning path.
	owner(path string) (ident, error)
	// lookup returns the uid of account name, and false when the container
	// has no such account.
	lookup(name string) (uint32, bool, error)
	// addAccount creates account name with id, its home and zsh.
	addAccount(name string, id ident, home string) error
	// become makes the process id, with its gid as its only group.
	become(id ident) error
	// serverReady reports whether name's server accepts connections; only
	// the account can reach it.
	serverReady(name string) bool
	// exec replaces the process with argv run in dir, with env.
	exec(dir string, argv, env []string) error
}

// run waits for the accounts to be set up, becomes the account, waits for its
// server when asked, then execs the command. It returns only on failure.
func run(h host, o options, base []string, poll time.Duration, logf func(string, ...any)) error {
	waitFor(func() bool { return h.exists(o.ready) }, poll, func() {
		logf("waiting for the server container to set the accounts up (%s)", o.ready)
	})
	home := filepath.Join(o.homeRoot, o.account)
	id, err := h.owner(home)
	if err != nil {
		return err
	}
	uid, found, err := h.lookup(o.account)
	switch {
	case err != nil:
		return err
	case !found:
		if err := h.addAccount(o.account, id, home); err != nil {
			return fmt.Errorf("creating account %s: %w", o.account, err)
		}
	case uid != id.uid:
		return fmt.Errorf("account %s has uid %d here, but its home %s belongs to uid %d", o.account, uid, home, id.uid)
	}
	if err := h.become(id); err != nil {
		return err
	}
	if o.waitServer {
		waitFor(func() bool { return h.serverReady(o.account) }, poll, func() {
			logf("waiting for the Rex server of %s", o.account)
		})
	}
	dir := o.cwd
	if dir == "" {
		dir = home
	}
	return h.exec(dir, o.command, environ(base, o, id.uid, home))
}

// waitFor returns once cond holds, polling every poll and calling waiting
// once if it does not hold at first.
func waitFor(cond func() bool, poll time.Duration, waiting func()) {
	if cond() {
		return
	}
	waiting()
	for !cond() {
		time.Sleep(poll)
	}
}

// environ returns base with the account's identity, its temporary directory
// and its Rex runtime directory set, and Rex's autostart off.
func environ(base []string, o options, uid uint32, home string) []string {
	tmp := fmt.Sprintf("/tmp/user/%d", uid)
	set := []string{
		"HOME=" + home,
		"USER=" + o.account,
		"LOGNAME=" + o.account,
		"SHELL=/usr/bin/zsh",
		"TMPDIR=" + tmp,
		"TMP=" + tmp,
		"REX_RUNTIME_DIR=" + account.RuntimeDir(o.runtimeRoot, o.account),
		"REX_AUTOSTART=false",
	}
	env := make([]string, 0, len(base)+len(set))
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.ContainsFunc(set, func(s string) bool { return strings.HasPrefix(s, name+"=") }) {
			env = append(env, kv)
		}
	}
	return append(env, set...)
}

type container struct{ runtimeRoot string }

func (container) exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (container) owner(path string) (ident, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return ident{}, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return ident{}, fmt.Errorf("%s: no owner", path)
	}
	return ident{uid: st.Uid, gid: st.Gid}, nil
}

func (container) lookup(name string) (uint32, bool, error) {
	u, err := user.Lookup(name)
	if errors.As(err, new(user.UnknownUserError)) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	return uint32(uid), true, err
}

func (container) addAccount(name string, id ident, home string) error {
	if _, err := user.LookupGroupId(strconv.FormatUint(uint64(id.gid), 10)); err != nil {
		if err := command("groupadd", "--gid", fmt.Sprint(id.gid), name); err != nil {
			return err
		}
	}
	return command("useradd", "--uid", fmt.Sprint(id.uid), "--gid", fmt.Sprint(id.gid),
		"--home-dir", home, "--no-create-home", "--shell", "/usr/bin/zsh", name)
}

func command(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c container) serverReady(name string) bool { return account.ServerReady(c.runtimeRoot, name) }

func (container) become(id ident) error {
	if err := syscall.Setgroups([]int{int(id.gid)}); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if err := syscall.Setgid(int(id.gid)); err != nil {
		return fmt.Errorf("setgid: %w", err)
	}
	if err := syscall.Setuid(int(id.uid)); err != nil {
		return fmt.Errorf("setuid: %w", err)
	}
	return nil
}

func (container) exec(dir string, argv, env []string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	return syscall.Exec(path, argv, env)
}

// Main runs "kuberexctl run" with args, the words after "run", and returns
// its exit code; it returns only on failure, once the command has replaced it
// otherwise.
func Main(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	o := options{homeRoot: "/home"}
	fs.StringVar(&o.account, "account", "", "the account to run as; required")
	fs.StringVar(&o.cwd, "cwd", "", "the directory to start in; the account's home when empty")
	fs.BoolVar(&o.waitServer, "wait-server", false, "wait for the account's Rex server to accept connections")
	fs.StringVar(&o.runtimeRoot, "runtime-root", "/run/rex", "the login server's runtime root")
	fs.StringVar(&o.ready, "ready", DefaultReady, "the file the server container creates once the accounts are set up")
	_ = fs.Parse(args)
	o.command = fs.Args()
	if o.account == "" || len(o.command) == 0 {
		fmt.Fprintln(os.Stderr, "usage: kuberexctl run --account NAME [flags] -- COMMAND [ARG...]")
		return 2
	}
	logf := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "kuberexctl run: "+format+"\n", a...)
	}
	err := run(container{runtimeRoot: o.runtimeRoot}, o, os.Environ(), time.Second, logf)
	logf("%v", err)
	return 1
}

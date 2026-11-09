// Command kuberex-claude resumes the Claude Code session a Rex block ran when
// its Rex server stopped, once the server has restored the block.
//
//	kuberex-claude record   SessionStart hook: keeps the session for $REX_BLOCK
//	kuberex-claude forget   SessionEnd hook: drops it when the person quit
//	kuberex-claude resume   from the block's shell: runs claude --resume
//
// Sessions are kept in $XDG_STATE_HOME/kuberex/claude, by default
// ~/.local/state/kuberex/claude. Only an interactive session started outside
// any other Claude Code session is kept. A block's session is resumed at most
// once per Rex server, and never under the server it last ran under.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
)

// app holds what kuberex-claude reads from outside.
type app struct {
	getenv func(string) string
	proc   procfs
	store  store
	// liveBlocks returns the IDs of every block the Rex server holds.
	liveBlocks func() (map[string]bool, error)
}

// hookInput holds the fields of a Claude Code hook's input that
// kuberex-claude reads.
type hookInput struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Reason    string `json:"reason"`
}

func readHookInput(r io.Reader) (hookInput, error) {
	var in hookInput
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return in, fmt.Errorf("reading hook input: %w", err)
	}
	if in.SessionID == "" {
		return in, errors.New("hook input has no session_id")
	}
	return in, nil
}

// block returns $REX_BLOCK, or "" when it is unset or not a file name.
func (a app) block() string {
	b := a.getenv("REX_BLOCK")
	if b == "" || b == "." || b == ".." || filepath.Base(b) != b || b[0] == '.' {
		return ""
	}
	return b
}

// record keeps the session of the SessionStart hook input in, if the Claude
// Code process $CLAUDE_PID runs in a Rex block and is resumable, then drops
// the sessions of blocks the server no longer holds.
func (a app) record(r io.Reader) error {
	in, err := readHookInput(r)
	if err != nil {
		return err
	}
	block := a.block()
	if block == "" {
		return nil
	}
	pid, err := strconv.Atoi(a.getenv("CLAUDE_PID"))
	if err != nil {
		return fmt.Errorf("CLAUDE_PID: %w", err)
	}
	ok, err := resumable(a.proc, pid)
	if err != nil || !ok {
		return err
	}
	server, err := a.proc.rexServer(pid)
	if errors.Is(err, errNoServer) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := a.store.save(block, session{ID: in.SessionID, Cwd: in.Cwd, Server: server}); err != nil {
		return err
	}
	return a.prune()
}

// resumable reports whether Claude Code process pid runs an interactive
// session started outside any other Claude Code session.
func resumable(p procfs, pid int) (bool, error) {
	env, err := p.strings0(pid, "environ")
	if err != nil {
		return false, err
	}
	if slices.Contains(env, "CLAUDE_CODE_CHILD_SESSION=1") {
		return false, nil
	}
	args, err := p.strings0(pid, "cmdline")
	if err != nil {
		return false, err
	}
	for _, arg := range args[1:] {
		switch arg {
		case "--":
			return true, nil
		case "-p", "--print":
			return false, nil
		}
	}
	return true, nil
}

// prune drops the sessions of blocks the Rex server no longer holds.
func (a app) prune() error {
	blocks, err := a.store.blocks()
	if err != nil || len(blocks) == 0 {
		return err
	}
	live, err := a.liveBlocks()
	if err != nil {
		return fmt.Errorf("listing blocks: %w", err)
	}
	for _, b := range blocks {
		if !live[b] {
			if err := a.store.remove(b); err != nil {
				return err
			}
		}
	}
	return nil
}

// forget drops the block's session when the SessionEnd hook input in ends it
// at the person's request. A session ended by a signal, as at a restart, is
// kept.
func (a app) forget(r io.Reader) error {
	in, err := readHookInput(r)
	if err != nil {
		return err
	}
	if in.Reason != "prompt_input_exit" && in.Reason != "logout" {
		return nil
	}
	block := a.block()
	if block == "" {
		return nil
	}
	sess, err := a.store.load(block)
	if err != nil || sess == nil || sess.ID != in.SessionID {
		return err
	}
	return a.store.remove(block)
}

// resume returns the session to resume in the block whose shell is an
// ancestor of pid, or nil when there is none. A returned session is marked as
// resumed under the current server, so the next call returns nil.
func (a app) resume(pid int) (*session, error) {
	block := a.block()
	if block == "" {
		return nil, nil
	}
	sess, err := a.store.load(block)
	if err != nil || sess == nil {
		return nil, err
	}
	server, err := a.proc.rexServer(pid)
	if errors.Is(err, errNoServer) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sess.Server == server {
		return nil, nil
	}
	sess.Server = server
	if err := a.store.save(block, *sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// rexBlocks lists the blocks of every session of the Rex server at
// $REX_SERVER through the rex command.
func rexBlocks() (map[string]bool, error) {
	var sessions struct {
		Sessions []struct {
			ID string `json:"session_id"`
		} `json:"sessions"`
	}
	if err := rexJSON(&sessions, "ls", "--json"); err != nil {
		return nil, err
	}
	live := map[string]bool{}
	for _, s := range sessions.Sessions {
		var blocks struct {
			Blocks []struct {
				ID string `json:"block_id"`
			} `json:"blocks"`
		}
		if err := rexJSON(&blocks, "block", "ls", "-s", s.ID, "--json"); err != nil {
			return nil, err
		}
		for _, b := range blocks.Blocks {
			live[b.ID] = true
		}
	}
	return live, nil
}

func rexJSON(v any, args ...string) error {
	cmd := exec.Command("rex", append([]string{"--autostart=false"}, args...)...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("rex %v: %w", args, err)
	}
	return json.Unmarshal(out, v)
}

// execResume replaces this process with claude --resume for sess, in the
// directory the session started in.
func execResume(sess *session) error {
	if err := os.Chdir(sess.Cwd); err != nil {
		return fmt.Errorf("session %s: %w", sess.ID, err)
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "kuberex-claude: resuming Claude Code session %s in %s\n", sess.ID, sess.Cwd)
	return syscall.Exec(claude, []string{"claude", "--resume", sess.ID}, os.Environ())
}

func run(cmd string) error {
	dir, err := stateDir(os.Getenv)
	if err != nil {
		return err
	}
	a := app{
		getenv:     os.Getenv,
		proc:       procfs{os.DirFS("/proc")},
		store:      store{dir},
		liveBlocks: rexBlocks,
	}
	switch cmd {
	case "record":
		return a.record(os.Stdin)
	case "forget":
		return a.forget(os.Stdin)
	case "resume":
		sess, err := a.resume(os.Getpid())
		if err != nil || sess == nil {
			return err
		}
		return execResume(sess)
	}
	return errUsage
}

var errUsage = errors.New("usage: kuberex-claude record|forget|resume")

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, errUsage)
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "kuberex-claude:", err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

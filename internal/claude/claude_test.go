package claude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

const bootID = "8e2c1f5a-0000-4000-8000-000000000001"

// fakeProc is a /proc of processes, each given as its parent, comm and start
// time, with optional environ and cmdline.
type fakeProc map[int]fakeProcess

type fakeProcess struct {
	ppid    int
	comm    string
	start   int
	environ []string
	cmdline []string
}

func (fp fakeProc) fs() procfs {
	m := fstest.MapFS{
		"sys/kernel/random/boot_id": {Data: []byte(bootID + "\n")},
	}
	for pid, p := range fp {
		dir := strconv.Itoa(pid)
		m[dir+"/stat"] = &fstest.MapFile{Data: []byte(statLine(pid, p.ppid, p.comm, p.start))}
		m[dir+"/environ"] = &fstest.MapFile{Data: []byte(nulJoin(p.environ))}
		m[dir+"/cmdline"] = &fstest.MapFile{Data: []byte(nulJoin(p.cmdline))}
	}
	return procfs{m}
}

func statLine(pid, ppid int, comm string, start int) string {
	return fmt.Sprintf("%d (%s) S %d %d %d 0 -1 4194560 100 0 0 0 1 2 0 0 20 0 1 0 %d 1000 200\n",
		pid, comm, ppid, ppid, ppid, start)
}

func nulJoin(s []string) string {
	var b strings.Builder
	for _, v := range s {
		b.WriteString(v)
		b.WriteByte(0)
	}
	return b.String()
}

const (
	pidRex    = 40
	pidShell  = 41
	pidClaude = 42
	pidHook   = 43
	pidNested = 44
	pidLone   = 50
)

// tree is a Rex server running a shell running Claude Code, whose Bash tool
// started a nested claude -p, and a claude outside any Rex server.
func tree() fakeProc {
	return fakeProc{
		1:         {ppid: 0, comm: "dumb-init", start: 1},
		pidRex:    {ppid: 1, comm: "rex", start: 9000},
		pidShell:  {ppid: pidRex, comm: "zsh", start: 9100},
		pidClaude: {ppid: pidShell, comm: "claude", start: 9200, environ: []string{"HOME=/home/alice"}, cmdline: []string{"claude", "--model", "opus"}},
		pidHook:   {ppid: pidClaude, comm: "kuberexctl", start: 9300},
		pidNested: {ppid: pidClaude, comm: "claude", start: 9400, environ: []string{"CLAUDECODE=1", "CLAUDE_CODE_CHILD_SESSION=1"}, cmdline: []string{"claude", "-p", "hi"}},
		pidLone:   {ppid: 1, comm: "claude", start: 9500, cmdline: []string{"claude"}},
	}
}

var serverID = bootID + "/40/9000"

func newApp(t *testing.T, proc fakeProc, env map[string]string, live ...string) app {
	t.Helper()
	return app{
		getenv: func(k string) string { return env[k] },
		proc:   proc.fs(),
		store:  store{t.TempDir()},
		liveBlocks: func() (map[string]bool, error) {
			m := map[string]bool{}
			for _, b := range live {
				m[b] = true
			}
			return m, nil
		},
	}
}

func hookJSON(id, cwd, reason string) *strings.Reader {
	return strings.NewReader(fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"x","reason":%q}`, id, cwd, reason))
}

func TestParseStat(t *testing.T) {
	p, err := parseStat("77 (a (b) c) S 12 77 77 0 -1 0 0 0 0 0 0 0 0 0 20 0 1 0 31337 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := (process{pid: 77, ppid: 12, comm: "a (b) c", start: "31337"}); p != want {
		t.Errorf("parseStat = %+v, want %+v", p, want)
	}
	for _, s := range []string{"", "77 a S 1", "77 (x) S 1 2 3"} {
		if _, err := parseStat(s); err == nil {
			t.Errorf("parseStat(%q) accepted a malformed line", s)
		}
	}
}

func TestRexServer(t *testing.T) {
	p := tree().fs()
	for _, pid := range []int{pidRex, pidHook} {
		got, err := p.rexServer(pid)
		if err != nil || got != serverID {
			t.Errorf("rexServer(%d) = %q, %v; want %q", pid, got, err, serverID)
		}
	}
	if _, err := p.rexServer(pidLone); !errors.Is(err, errNoServer) {
		t.Errorf("rexServer outside Rex = %v, want errNoServer", err)
	}
}

func TestRecord(t *testing.T) {
	env := map[string]string{"REX_BLOCK": "block:a", "CLAUDE_PID": strconv.Itoa(pidClaude)}
	a := newApp(t, tree(), env, "block:a")
	if err := a.record(hookJSON("s1", "/home/alice/proj", "")); err != nil {
		t.Fatal(err)
	}
	got, err := a.store.load("block:a")
	if err != nil {
		t.Fatal(err)
	}
	if want := (session{ID: "s1", Cwd: "/home/alice/proj", Server: serverID}); got == nil || *got != want {
		t.Errorf("recorded %+v, want %+v", got, want)
	}
}

func TestRecordSkips(t *testing.T) {
	proc := tree()
	print := proc[pidClaude]
	print.cmdline = []string{"claude", "--model", "opus", "--print", "hi"}
	proc[pidClaude+100] = print
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"outside a block", map[string]string{"CLAUDE_PID": strconv.Itoa(pidClaude)}},
		{"block names a path", map[string]string{"REX_BLOCK": "../x", "CLAUDE_PID": strconv.Itoa(pidClaude)}},
		{"nested session", map[string]string{"REX_BLOCK": "block:a", "CLAUDE_PID": strconv.Itoa(pidNested)}},
		{"print mode", map[string]string{"REX_BLOCK": "block:a", "CLAUDE_PID": strconv.Itoa(pidClaude + 100)}},
		{"outside a Rex server", map[string]string{"REX_BLOCK": "block:a", "CLAUDE_PID": strconv.Itoa(pidLone)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, proc, tt.env, "block:a")
			if err := a.record(hookJSON("s1", "/home/alice", "")); err != nil {
				t.Fatal(err)
			}
			if blocks, _ := a.store.blocks(); len(blocks) != 0 {
				t.Errorf("recorded sessions for %q", blocks)
			}
		})
	}
}

func TestRecordPrunesClosedBlocks(t *testing.T) {
	env := map[string]string{"REX_BLOCK": "block:a", "CLAUDE_PID": strconv.Itoa(pidClaude)}
	a := newApp(t, tree(), env, "block:a", "block:b")
	for _, b := range []string{"block:b", "block:closed"} {
		if err := a.store.save(b, session{ID: "old"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.record(hookJSON("s1", "/home/alice", "")); err != nil {
		t.Fatal(err)
	}
	got, err := a.store.blocks()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"block:a", "block:b"}; !slices.Equal(got, want) {
		t.Errorf("blocks after prune = %q, want %q", got, want)
	}
}

func TestForget(t *testing.T) {
	tests := []struct {
		id, reason string
		kept       bool
	}{
		{"s1", "prompt_input_exit", false},
		{"s1", "logout", false},
		{"s1", "other", true},
		{"s1", "clear", true},
		{"s2", "prompt_input_exit", true},
	}
	for _, tt := range tests {
		t.Run(tt.id+"/"+tt.reason, func(t *testing.T) {
			a := newApp(t, tree(), map[string]string{"REX_BLOCK": "block:a"})
			if err := a.store.save("block:a", session{ID: "s1", Cwd: "/home/alice"}); err != nil {
				t.Fatal(err)
			}
			if err := a.forget(hookJSON(tt.id, "/home/alice", tt.reason)); err != nil {
				t.Fatal(err)
			}
			got, err := a.store.load("block:a")
			if err != nil {
				t.Fatal(err)
			}
			if kept := got != nil; kept != tt.kept {
				t.Errorf("kept = %v, want %v", kept, tt.kept)
			}
		})
	}
}

func TestResume(t *testing.T) {
	a := newApp(t, tree(), map[string]string{"REX_BLOCK": "block:a"})
	if err := a.store.save("block:a", session{ID: "s1", Cwd: "/home/alice/proj", Server: bootID + "/40/100"}); err != nil {
		t.Fatal(err)
	}
	got, err := a.resume(pidShell)
	if err != nil {
		t.Fatal(err)
	}
	if want := (session{ID: "s1", Cwd: "/home/alice/proj", Server: serverID}); got == nil || *got != want {
		t.Fatalf("resume = %+v, want %+v", got, want)
	}
	again, err := a.resume(pidShell)
	if err != nil || again != nil {
		t.Errorf("second resume under the same server = %+v, %v; want none", again, err)
	}
}

func TestResumeNothing(t *testing.T) {
	tests := []struct {
		name  string
		block string
		saved *session
		pid   int
	}{
		{"no session", "block:a", nil, pidShell},
		{"server that recorded it", "block:a", &session{ID: "s1", Server: serverID}, pidShell},
		{"outside a Rex server", "block:a", &session{ID: "s1", Server: "gone"}, pidLone},
		{"outside a block", "", &session{ID: "s1", Server: "gone"}, pidShell},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, tree(), map[string]string{"REX_BLOCK": tt.block})
			if tt.saved != nil {
				if err := a.store.save("block:a", *tt.saved); err != nil {
					t.Fatal(err)
				}
			}
			got, err := a.resume(tt.pid)
			if err != nil || got != nil {
				t.Errorf("resume = %+v, %v; want none", got, err)
			}
		})
	}
}

func TestStoreBlocksSkipsOtherFiles(t *testing.T) {
	s := store{t.TempDir()}
	if err := s.save("block:a", session{ID: "s1"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".tmp-123", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(s.dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.blocks()
	if err != nil || !slices.Equal(got, []string{"block:a"}) {
		t.Errorf("blocks = %q, %v; want [block:a]", got, err)
	}
}

func TestStateDir(t *testing.T) {
	tests := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"HOME": "/home/alice"}, "/home/alice/.local/state/kuberex/claude"},
		{map[string]string{"HOME": "/home/alice", "XDG_STATE_HOME": "/var/state"}, "/var/state/kuberex/claude"},
	}
	for _, tt := range tests {
		got, err := stateDir(func(k string) string { return tt.env[k] })
		if err != nil || got != tt.want {
			t.Errorf("stateDir(%v) = %q, %v; want %q", tt.env, got, err, tt.want)
		}
	}
	if _, err := stateDir(func(string) string { return "" }); err == nil {
		t.Error("stateDir without HOME succeeded")
	}
}

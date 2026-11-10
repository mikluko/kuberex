package runas

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeHost is a container where ready appears after readyAfter checks, and
// the server after serverAfter. accounts maps the names it knows to uids.
type fakeHost struct {
	readyAfter, serverAfter int
	home                    ident
	accounts                map[string]uint32
	added                   []string
	became                  *ident
	execed                  struct {
		dir  string
		argv []string
		env  []string
	}
	serverChecks int
	// checkedAsRoot counts server checks made before become.
	checkedAsRoot int
}

func (f *fakeHost) exists(string) bool {
	f.readyAfter--
	return f.readyAfter < 0
}

func (f *fakeHost) owner(string) (ident, error) { return f.home, nil }

func (f *fakeHost) lookup(name string) (uint32, bool, error) {
	uid, ok := f.accounts[name]
	return uid, ok, nil
}

func (f *fakeHost) addAccount(name string, id ident, home string) error {
	f.added = append(f.added, name+" "+home)
	f.accounts[name] = id.uid
	return nil
}

func (f *fakeHost) become(id ident) error {
	f.became = &id
	return nil
}

func (f *fakeHost) serverReady(string) bool {
	if f.became == nil {
		f.checkedAsRoot++
	}
	f.serverChecks++
	return f.serverChecks > f.serverAfter
}

func (f *fakeHost) exec(dir string, argv, env []string) error {
	f.execed.dir, f.execed.argv, f.execed.env = dir, argv, env
	return nil
}

var city = options{
	account:     "city",
	homeRoot:    "/home",
	runtimeRoot: "/run/rex",
	ready:       "/run/kuberex/ready",
	command:     []string{"gc", "supervisor", "run"},
}

func runCity(t *testing.T, h *fakeHost, o options) []string {
	t.Helper()
	var logs []string
	logf := func(format string, a ...any) { logs = append(logs, format) }
	if err := run(h, o, []string{"PATH=/usr/bin", "HOME=/root", "TZ=UTC"}, time.Millisecond, logf); err != nil {
		t.Fatal(err)
	}
	return logs
}

func TestRunCreatesTheAccountFromItsHome(t *testing.T) {
	h := &fakeHost{readyAfter: 2, home: ident{uid: 1001, gid: 1001}, accounts: map[string]uint32{}}
	logs := runCity(t, h, city)
	if want := []string{"city /home/city"}; !reflect.DeepEqual(h.added, want) {
		t.Errorf("added %q, want %q", h.added, want)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "waiting for the server container") {
		t.Errorf("logs = %q, want one line about waiting for the accounts", logs)
	}
	if h.became == nil || *h.became != h.home || h.execed.dir != "/home/city" || !reflect.DeepEqual(h.execed.argv, city.command) {
		t.Errorf("exec(%v, %q, %q), want the command as uid 1001 in /home/city", h.became, h.execed.dir, h.execed.argv)
	}
	if h.serverChecks != 0 {
		t.Error("waited for the server without --wait-server")
	}
}

func TestRunKeepsAMatchingAccount(t *testing.T) {
	h := &fakeHost{home: ident{uid: 1001, gid: 1001}, accounts: map[string]uint32{"city": 1001}}
	o := city
	o.cwd = "/home/city/city"
	runCity(t, h, o)
	if len(h.added) != 0 {
		t.Errorf("added %q to a container that has the account", h.added)
	}
	if h.execed.dir != "/home/city/city" {
		t.Errorf("dir = %q, want the cwd", h.execed.dir)
	}
}

func TestRunRefusesAnAccountOfAnotherUID(t *testing.T) {
	h := &fakeHost{home: ident{uid: 1001, gid: 1001}, accounts: map[string]uint32{"city": 1000}}
	err := run(h, city, nil, time.Millisecond, func(string, ...any) {})
	if err == nil || !strings.Contains(err.Error(), "uid 1000") {
		t.Fatalf("run = %v, want the uid mismatch", err)
	}
	if h.execed.argv != nil {
		t.Error("ran the command anyway")
	}
}

func TestRunWaitsForTheServer(t *testing.T) {
	h := &fakeHost{serverAfter: 3, home: ident{uid: 1001, gid: 1001}, accounts: map[string]uint32{"city": 1001}}
	o := city
	o.waitServer = true
	runCity(t, h, o)
	if h.serverChecks != 4 {
		t.Errorf("server checked %d times, want until it was ready", h.serverChecks)
	}
	if h.checkedAsRoot != 0 {
		t.Error("checked the server as root, which cannot reach the account's runtime directory")
	}
}

func TestEnviron(t *testing.T) {
	env := environ([]string{"PATH=/usr/bin", "HOME=/root", "TZ=UTC", "REX_AUTOSTART=true"}, city, 1001, "/home/city")
	for _, want := range []string{
		"PATH=/usr/bin",
		"TZ=UTC",
		"HOME=/home/city",
		"USER=city",
		"LOGNAME=city",
		"SHELL=/usr/bin/zsh",
		"TMPDIR=/tmp/user/1001",
		"TMP=/tmp/user/1001",
		"REX_RUNTIME_DIR=/run/rex/accounts/city",
		"REX_AUTOSTART=false",
	} {
		if !slices.Contains(env, want) {
			t.Errorf("environ lacks %s: %q", want, env)
		}
	}
	for _, gone := range []string{"HOME=/root", "REX_AUTOSTART=true"} {
		if slices.Contains(env, gone) {
			t.Errorf("environ keeps %s", gone)
		}
	}
}

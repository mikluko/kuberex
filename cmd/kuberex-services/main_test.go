package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fakeHost records the rex calls made as each account and answers rex ls with
// ls. Its server becomes ready after startServer, or at once when up is set.
type fakeHost struct {
	up      bool
	starts  int
	ls      string
	calls   [][]string
	failRex error
}

func (f *fakeHost) serverReady(string) bool { return f.up }

func (f *fakeHost) startServer(string) error {
	f.starts++
	f.up = true
	return nil
}

func (f *fakeHost) rex(account string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{account}, args...))
	if f.failRex != nil {
		return nil, f.failRex
	}
	if args[0] == "ls" {
		return []byte(f.ls), nil
	}
	return nil, nil
}

var gc = service{Account: "city", Label: "gc", Cwd: "/home/city/city", Command: []string{"gc", "supervisor", "run"}}

func TestStartServiceFresh(t *testing.T) {
	h := &fakeHost{ls: `{"sessions":[]}`}
	if err := startService(h, gc, time.Second); err != nil {
		t.Fatal(err)
	}
	if h.starts != 1 {
		t.Errorf("servers started = %d, want 1", h.starts)
	}
	want := [][]string{
		{"city", "ls", "--json"},
		{"city", "new", "gc", "--keep-open", "--cwd", "/home/city/city", "--", "gc", "supervisor", "run"},
	}
	if !reflect.DeepEqual(h.calls, want) {
		t.Errorf("rex calls = %q, want %q", h.calls, want)
	}
}

func TestStartServiceReplacesRestoredSession(t *testing.T) {
	h := &fakeHost{up: true, ls: `{"sessions":[
		{"session_id":"session:a","label":"gc"},
		{"session_id":"session:b","label":"work"},
		{"session_id":"session:c","label":"gc"}]}`}
	if err := startService(h, gc, time.Second); err != nil {
		t.Fatal(err)
	}
	if h.starts != 0 {
		t.Errorf("started a server while one was running")
	}
	if want := []string{"city", "kill", "session:a", "session:c"}; !reflect.DeepEqual(h.calls[1], want) {
		t.Errorf("kill call = %q, want %q", h.calls[1], want)
	}
	if h.calls[2][1] != "new" {
		t.Errorf("third call = %q, want new", h.calls[2])
	}
}

func TestStartServiceWithoutCwd(t *testing.T) {
	h := &fakeHost{up: true, ls: `{"sessions":[]}`}
	s := gc
	s.Cwd = ""
	if err := startService(h, s, time.Second); err != nil {
		t.Fatal(err)
	}
	if want := []string{"city", "new", "gc", "--keep-open", "--", "gc", "supervisor", "run"}; !reflect.DeepEqual(h.calls[1], want) {
		t.Errorf("new call = %q, want %q", h.calls[1], want)
	}
}

func TestStartServiceRexFails(t *testing.T) {
	h := &fakeHost{up: true, failRex: errors.New("boom")}
	if err := startService(h, gc, time.Second); err == nil {
		t.Error("startService ignored a failing rex")
	}
}

type deadHost struct{ fakeHost }

func (*deadHost) serverReady(string) bool { return false }

func TestEnsureServerTimesOut(t *testing.T) {
	h := &deadHost{}
	err := ensureServer(h, "city", 250*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not start") {
		t.Errorf("ensureServer = %v, want a timeout", err)
	}
}

func TestParseServices(t *testing.T) {
	svcs, err := parseServices([]byte(`[{"account":"city","label":"gc","command":["gc","supervisor","run"]}]`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []service{{Account: "city", Label: "gc", Command: []string{"gc", "supervisor", "run"}}}; !reflect.DeepEqual(svcs, want) {
		t.Errorf("parseServices = %+v, want %+v", svcs, want)
	}
	for _, src := range []string{
		`[{"label":"gc","command":["gc"]}]`,
		`[{"account":"city","command":["gc"]}]`,
		`[{"account":"city","label":"gc","command":[]}]`,
		`{"account":"city"}`,
	} {
		if _, err := parseServices([]byte(src)); err == nil {
			t.Errorf("parseServices(%s) accepted it", src)
		}
	}
}

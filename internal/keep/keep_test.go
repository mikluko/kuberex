package keep

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// fakeHost holds the accounts whose servers are up. A started server comes up
// unless its account is in dead; a start fails for an account in broken.
type fakeHost struct {
	up      map[string]bool
	dead    map[string]bool
	broken  map[string]bool
	started []string
}

func (f *fakeHost) serverReady(name string) bool { return f.up[name] }

func (f *fakeHost) startServer(name string) error {
	f.started = append(f.started, name)
	if f.broken[name] {
		return errors.New("boom")
	}
	if !f.dead[name] {
		f.up[name] = true
	}
	return nil
}

func nolog(string, ...any) {}

func TestPassStartsOnlyServersThatAreDown(t *testing.T) {
	h := &fakeHost{up: map[string]bool{"alice": true}}
	if err := pass(context.Background(), h, []string{"alice", "city"}, time.Second, time.Millisecond, nolog); err != nil {
		t.Fatal(err)
	}
	if want := []string{"city"}; !reflect.DeepEqual(h.started, want) {
		t.Errorf("started %q, want %q", h.started, want)
	}
}

func TestPassReportsServersThatFail(t *testing.T) {
	h := &fakeHost{up: map[string]bool{}, dead: map[string]bool{"city": true}, broken: map[string]bool{"bob": true}}
	err := pass(context.Background(), h, []string{"bob", "city", "alice"}, 20*time.Millisecond, time.Millisecond, nolog)
	if err == nil {
		t.Fatal("pass = nil, want the failures of bob and city")
	}
	if !h.up["alice"] {
		t.Error("a failing account kept alice's server from starting")
	}
}

func TestPassStartsNothingOnceDone(t *testing.T) {
	h := &fakeHost{up: map[string]bool{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = pass(ctx, h, []string{"city"}, time.Second, time.Millisecond, nolog)
	if len(h.started) != 0 {
		t.Errorf("started %q after being stopped", h.started)
	}
}

// dyingHost has a server that is gone after answering life checks.
type dyingHost struct {
	life, left int
	starts     int
}

func (d *dyingHost) serverReady(string) bool {
	if d.left == 0 {
		return false
	}
	d.left--
	return true
}

func (d *dyingHost) startServer(string) error {
	d.starts++
	d.left = d.life
	return nil
}

func TestKeepStartsAServerAgain(t *testing.T) {
	h := &dyingHost{life: 3}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	keep(ctx, h, []string{"city"}, 10*time.Millisecond, time.Second, nolog)
	if h.starts < 2 {
		t.Errorf("started city %d times, want it started again once its server was gone", h.starts)
	}
}

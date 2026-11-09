package control

import (
	"errors"
	"reflect"
	"testing"
)

// fakeHost records the shutdown steps in order.
type fakeHost struct {
	running  []string
	failOn   string
	failMark bool
	steps    []string
}

func (f *fakeHost) markShutdown() error {
	f.steps = append(f.steps, "mark")
	if f.failMark {
		return errors.New("read-only")
	}
	return nil
}

func (f *fakeHost) servers() ([]string, error) { return f.running, nil }

func (f *fakeHost) stopServer(account string) error {
	f.steps = append(f.steps, "stop "+account)
	if account == f.failOn {
		return errors.New("refused")
	}
	return nil
}

func (f *fakeHost) sync() { f.steps = append(f.steps, "sync") }

func (f *fakeHost) terminate() error {
	f.steps = append(f.steps, "terminate")
	return nil
}

func lookupFake(uid uint32) (string, error) {
	names := map[uint32]string{1000: "city", 1001: "bao"}
	if n, ok := names[uid]; ok {
		return n, nil
	}
	return "", errors.New("unknown uid")
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name  string
		uid   uint32
		req   string
		start bool
	}{
		{"allowed account", 1000, "shutdown", true},
		{"account not allowed", 1001, "shutdown", false},
		{"unknown uid", 4242, "shutdown", false},
		{"unknown request", 1000, "restart", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newServer(parseAccounts("city"), lookupFake, &fakeHost{})
			reply, start := s.decide(tt.uid, tt.req)
			if start != tt.start {
				t.Fatalf("start = %v (%q), want %v", start, reply, tt.start)
			}
			if start != (reply == "ok") {
				t.Fatalf("reply %q does not match start %v", reply, start)
			}
		})
	}
}

func TestDecideOnlyFirstShutdownStarts(t *testing.T) {
	s := newServer(parseAccounts("city"), lookupFake, &fakeHost{})
	if _, start := s.decide(1000, "shutdown"); !start {
		t.Fatal("first request did not start a shutdown")
	}
	if reply, start := s.decide(1000, "shutdown"); start {
		t.Fatalf("second request started another shutdown: %q", reply)
	}
}

func TestShutdownMarksThenTerminatesWithoutStoppingServers(t *testing.T) {
	for _, failMark := range []bool{false, true} {
		h := &fakeHost{running: []string{"city"}, failMark: failMark}
		s := newServer(nil, lookupFake, h)
		if err := s.shutdown(); err != nil {
			t.Fatal(err)
		}
		want := []string{"mark", "terminate"}
		if !reflect.DeepEqual(h.steps, want) {
			t.Fatalf("failMark=%v: steps = %v, want %v", failMark, h.steps, want)
		}
	}
}

func TestStopServersStopsEveryServerThenSyncs(t *testing.T) {
	h := &fakeHost{running: []string{"city", "bao", "ops"}, failOn: "bao"}
	if failed := stopServers(h); failed != 1 {
		t.Fatalf("failed = %d, want 1", failed)
	}
	want := []string{"stop city", "stop bao", "stop ops", "sync"}
	if !reflect.DeepEqual(h.steps, want) {
		t.Fatalf("steps = %v, want %v", h.steps, want)
	}
}

func TestParseAccounts(t *testing.T) {
	got := parseAccounts(" city,bao  ops,")
	want := map[string]bool{"city": true, "bao": true, "ops": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseAccounts = %v, want %v", got, want)
	}
}

func TestRequestWithoutServer(t *testing.T) {
	_, err := request(t.TempDir()+"/control.sock", "shutdown")
	if err == nil || err.Error() != "shutdown is not enabled in this pod" {
		t.Fatalf("err = %v", err)
	}
}

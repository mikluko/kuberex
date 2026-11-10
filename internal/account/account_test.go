package account

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeDir(t *testing.T) {
	got := RuntimeDir("/run/rex", "alice")
	if want := "/run/rex/accounts/alice"; got != want {
		t.Errorf("RuntimeDir = %q, want %q", got, want)
	}
}

func TestServerSocket(t *testing.T) {
	got := ServerSocket("/run/rex", "alice")
	if want := "/run/rex/accounts/alice/server.sock"; got != want {
		t.Errorf("ServerSocket = %q, want %q", got, want)
	}
}

func TestServerReady(t *testing.T) {
	// Unix socket paths are limited to ~100 bytes, so avoid t.TempDir's long name.
	root, err := os.MkdirTemp("", "acct")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })

	if ServerReady(root, "alice") {
		t.Fatal("ServerReady = true with no socket")
	}

	if err := os.MkdirAll(RuntimeDir(root, "alice"), 0o700); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", ServerSocket(root, "alice"))
	if err != nil {
		t.Fatal(err)
	}
	if !ServerReady(root, "alice") {
		t.Error("ServerReady = false with a listener bound")
	}
	if ServerReady(root, "bob") {
		t.Error("ServerReady = true for an account with no socket")
	}

	l.Close()
	os.Remove(filepath.Join(RuntimeDir(root, "alice"), "server.sock"))
	if ServerReady(root, "alice") {
		t.Error("ServerReady = true after the listener closed")
	}
}

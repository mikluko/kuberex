package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no subcommand", nil, 2},
		{"unknown subcommand", []string{"restart"}, 2},
		{"accounts without a file", []string{"accounts"}, 2},
		{"claude without a verb", []string{"claude"}, 2},
		{"services without a file", []string{"services"}, 2},
		{"keep-servers without accounts", []string{"keep-servers"}, 2},
		{"run without an account", []string{"run", "--", "true"}, 2},
		{"run without a command", []string{"run", "--account", "city"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.args); got != tt.want {
				t.Fatalf("run(%q) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestRunAccounts(t *testing.T) {
	f := filepath.Join(t.TempDir(), "accounts.jsonc")
	if err := os.WriteFile(f, []byte(`{"version": 1, "accounts": {"city": {}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"accounts", f}); got != 0 {
		t.Fatalf("run(accounts) = %d, want 0", got)
	}
}

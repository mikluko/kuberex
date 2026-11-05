package main

import (
	"slices"
	"testing"
)

func TestAccounts(t *testing.T) {
	src := []byte(`{
  // HuJSON, as the login server reads it
  "version": 1,
  "accounts": {
    "mikluko": { "principals": ["tailscale:ml@example.com"] },
    "root": { "principals": ["tailscale:ml@example.com"] },
    "*": { "principals": ["tailscale:*"] },
    "alice": { "principals": ["tailscale:alice@example.com"] },
  },
  "defaults": { "tailscale:ml@example.com": "mikluko" },
}`)
	got, err := accounts(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"mikluko", "alice"}; !slices.Equal(got, want) {
		t.Errorf("accounts = %q, want %q", got, want)
	}
}

func TestAccountsNone(t *testing.T) {
	got, err := accounts([]byte(`{"version": 1}`))
	if err != nil || got != nil {
		t.Errorf("accounts = %q, %v; want none", got, err)
	}
}

func TestAccountsMalformed(t *testing.T) {
	if _, err := accounts([]byte(`{"accounts": [}`)); err == nil {
		t.Error("accounts accepted a malformed file")
	}
}

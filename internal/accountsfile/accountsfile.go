// Package accountsfile prints the accounts a Rex login server's accounts file
// names, one per line in file order, for kuberex to create.
//
// The file is HuJSON, as the login server reads it. "*" and "root" are not
// printed: "*" stands for every regular account and root always exists.
package accountsfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tailscale/hujson"
)

func accounts(src []byte) ([]string, error) {
	std, err := hujson.Standardize(src)
	if err != nil {
		return nil, err
	}
	var f struct {
		Accounts json.RawMessage `json:"accounts"`
	}
	if err := json.Unmarshal(std, &f); err != nil {
		return nil, err
	}
	if len(f.Accounts) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(f.Accounts))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var names []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name := tok.(string)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
		if name != "*" && name != "root" {
			names = append(names, name)
		}
	}
	return names, nil
}

// Main runs "kuberexctl accounts" with args, the words after "accounts", and
// returns its exit code.
func Main(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: kuberexctl accounts ACCOUNTS-FILE")
		return 2
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kuberexctl accounts:", err)
		return 1
	}
	names, err := accounts(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kuberexctl accounts: %s: %v\n", args[0], err)
		return 1
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return 0
}

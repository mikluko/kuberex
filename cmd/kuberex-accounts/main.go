// Command kuberex-accounts prints the accounts a Rex login server's accounts
// file names, one per line in file order, for kuberex to create.
//
// The file is HuJSON, as the login server reads it. "*" and "root" are not
// printed: "*" stands for every regular account and root always exists.
package main

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

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: kuberex-accounts ACCOUNTS-FILE")
		os.Exit(2)
	}
	src, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kuberex-accounts:", err)
		os.Exit(1)
	}
	names, err := accounts(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kuberex-accounts: %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	for _, n := range names {
		fmt.Println(n)
	}
}

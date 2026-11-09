package claude

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// session is the Claude Code session a Rex block ran last.
type session struct {
	ID  string `json:"session_id"`
	Cwd string `json:"cwd"`
	// Server is the rexServer identity of the server the block ran under
	// when the session was last started or resumed.
	Server string `json:"server"`
}

// store keeps one session per Rex block, in a file named after the block.
type store struct{ dir string }

func stateDir(getenv func(string) string) (string, error) {
	base := getenv("XDG_STATE_HOME")
	if base == "" {
		home := getenv("HOME")
		if home == "" {
			return "", errors.New("neither XDG_STATE_HOME nor HOME is set")
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "kuberex", "claude"), nil
}

func (s store) path(block string) string {
	return filepath.Join(s.dir, block+".json")
}

// load returns the session of block, or nil when it has none.
func (s store) load(block string) (*session, error) {
	b, err := os.ReadFile(s.path(block))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sess session
	if err := json.Unmarshal(b, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// save replaces the session of block atomically.
func (s store) save(block string, sess session) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	err = errors.Join(err, f.Close())
	if err == nil {
		err = os.Rename(f.Name(), s.path(block))
	}
	if err != nil {
		return errors.Join(err, os.Remove(f.Name()))
	}
	return nil
}

func (s store) remove(block string) error {
	err := os.Remove(s.path(block))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// blocks lists the blocks that have a session.
func (s store) blocks() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var blocks []string
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		blocks = append(blocks, strings.TrimSuffix(name, ".json"))
	}
	return blocks, nil
}

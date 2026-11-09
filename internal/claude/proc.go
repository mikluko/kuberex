package claude

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

// errNoServer reports a process that no Rex server is an ancestor of.
var errNoServer = errors.New("no Rex server among its ancestors")

// procfs reads processes from a Linux /proc filesystem.
type procfs struct{ fsys fs.FS }

type process struct {
	pid, ppid int
	comm      string
	// start is the process's start time in clock ticks after boot.
	start string
}

func (p procfs) stat(pid int) (process, error) {
	b, err := fs.ReadFile(p.fsys, strconv.Itoa(pid)+"/stat")
	if err != nil {
		return process{}, err
	}
	return parseStat(string(b))
}

// parseStat parses /proc/<pid>/stat. The comm field may itself hold spaces
// and parentheses, so it ends at the last ')'.
func parseStat(s string) (process, error) {
	open, end := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open < 0 || end < open {
		return process{}, fmt.Errorf("malformed stat %q", s)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(s[:open]))
	if err != nil {
		return process{}, fmt.Errorf("malformed stat %q", s)
	}
	rest := strings.Fields(s[end+1:])
	if len(rest) <= statStart {
		return process{}, fmt.Errorf("malformed stat %q", s)
	}
	ppid, err := strconv.Atoi(rest[statPPID])
	if err != nil {
		return process{}, fmt.Errorf("malformed stat %q", s)
	}
	return process{pid: pid, ppid: ppid, comm: s[open+1 : end], start: rest[statStart]}, nil
}

// statPPID and statStart index ppid and starttime, fields 4 and 22 of
// proc_pid_stat(5), among the fields that follow comm.
const (
	statPPID  = 1
	statStart = 19
)

// rexServer identifies the nearest Rex server among the ancestors of pid, pid
// included, by boot, process ID and start time: a restarted server never has
// the identity of the one before it.
func (p procfs) rexServer(pid int) (string, error) {
	for pid > 0 {
		proc, err := p.stat(pid)
		if err != nil {
			return "", err
		}
		if proc.comm == "rex" {
			boot, err := fs.ReadFile(p.fsys, "sys/kernel/random/boot_id")
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s/%d/%s", bytes.TrimSpace(boot), proc.pid, proc.start), nil
		}
		pid = proc.ppid
	}
	return "", errNoServer
}

// strings0 reads a file of NUL-terminated strings, such as cmdline or environ.
func (p procfs) strings0(pid int, name string) ([]string, error) {
	b, err := fs.ReadFile(p.fsys, strconv.Itoa(pid)+"/"+name)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00"), nil
}

// Command record-base-bump records a base image bump in a Keep a Changelog
// file.
//
// With [Unreleased] empty, the bump is cut as the next patch version, dated,
// and the new version is printed. Otherwise the entry joins [Unreleased] under
// Changed and nothing is printed: a pending release carries the bump.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var types = []string{"Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"}

type heading struct {
	level int
	text  string
	line  int
}

func headings(src []byte) []heading {
	doc := goldmark.New().Parser().Parse(text.NewReader(src))
	var hs []heading
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := n.(*ast.Heading)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}
		seg := h.Lines().At(0)
		hs = append(hs, heading{
			level: h.Level,
			text:  strings.TrimSpace(string(h.Lines().Value(src))),
			line:  bytes.Count(src[:seg.Start], []byte("\n")),
		})
		return ast.WalkSkipChildren, nil
	})
	return hs
}

// lastText returns the index of the last non-blank line in lines[start:end],
// or -1 when there is none.
func lastText(lines []string, start, end int) int {
	for n := end - 1; n >= start; n-- {
		if strings.TrimSpace(lines[n]) != "" {
			return n
		}
	}
	return -1
}

func nextPatch(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("version %q is not MAJOR.MINOR.PATCH", version)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("version %q is not MAJOR.MINOR.PATCH", version)
	}
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1), nil
}

func splitLines(src []byte) []string {
	lines := strings.SplitAfter(string(src), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// record returns src with entry recorded, and the version cut, or "" when the
// entry joined [Unreleased] instead.
func record(src []byte, current, entry, date string) ([]byte, string, error) {
	lines := splitLines(src)
	hs := headings(src)
	k := slices.IndexFunc(hs, func(h heading) bool { return h.level == 2 && h.text == "[Unreleased]" })
	if k < 0 {
		return nil, "", fmt.Errorf("no ## [Unreleased] heading")
	}
	top := hs[k].line
	end := len(lines)
	var section []heading
	for _, h := range hs[k+1:] {
		if h.level <= 2 {
			end = h.line
			break
		}
		if h.level == 3 {
			section = append(section, h)
		}
	}
	bullet := "- " + entry + "\n"
	join := func(at int, add ...string) []byte {
		return []byte(strings.Join(slices.Insert(lines, at, add...), ""))
	}

	last := lastText(lines, top+1, end)
	if last < 0 {
		version, err := nextPatch(current)
		if err != nil {
			return nil, "", err
		}
		return join(top+1, "\n", "## ["+version+"] - "+date+"\n", "\n", "### Changed\n", "\n", bullet), version, nil
	}

	c := slices.IndexFunc(section, func(h heading) bool { return h.text == "Changed" })
	if c >= 0 {
		stop := end
		if c+1 < len(section) {
			stop = section[c+1].line
		}
		if at := lastText(lines, section[c].line+1, stop); at >= 0 {
			return join(at+1, bullet), "", nil
		}
		return join(section[c].line+1, "\n", bullet), "", nil
	}
	changed := slices.Index(types, "Changed")
	for _, h := range section {
		if slices.Index(types, h.text) > changed {
			return join(h.line, "### Changed\n", "\n", bullet, "\n"), "", nil
		}
	}
	return join(last+1, "\n", "### Changed\n", "\n", bullet), "", nil
}

func main() {
	path := flag.String("changelog", "CHANGELOG.md", "path to the changelog")
	current := flag.String("current", "", "version of the newest entry")
	entry := flag.String("entry", "", "bullet text, without the dash")
	date := flag.String("date", "", "release date, YYYY-MM-DD")
	flag.Parse()
	if *current == "" || *entry == "" || *date == "" {
		flag.Usage()
		os.Exit(2)
	}
	src, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, version, err := record(src, *current, *entry, *date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", *path, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*path, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if version != "" {
		fmt.Println(version)
	}
}

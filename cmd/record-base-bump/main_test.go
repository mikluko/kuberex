package main

import (
	"strings"
	"testing"
)

const (
	head     = "# Changelog\n\nPreamble.\n\n## [Unreleased]\n"
	released = "\n## [0.1.0] - 2026-10-30\n\n### Added\n\n- Rex server.\n"
	entry    = "Base image is `b:2`."
)

func run(t *testing.T, src string) (string, string) {
	t.Helper()
	out, version, err := record([]byte(src), "0.1.0", entry, "2026-11-03")
	if err != nil {
		t.Fatal(err)
	}
	return string(out), version
}

func TestEmptyUnreleasedCutsNextPatch(t *testing.T) {
	out, version := run(t, head+released)
	if version != "0.1.1" {
		t.Errorf("version = %q, want 0.1.1", version)
	}
	want := head + "\n## [0.1.1] - 2026-11-03\n\n### Changed\n\n- Base image is `b:2`.\n" + released
	if out != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

func TestPendingChangedGetsTheBullet(t *testing.T) {
	out, version := run(t, head+"\n### Changed\n\n- Other.\n\n### Fixed\n\n- Bug.\n"+released)
	if version != "" {
		t.Errorf("version = %q, want none", version)
	}
	if !strings.Contains(out, "- Other.\n- Base image is `b:2`.\n\n### Fixed") {
		t.Errorf("bullet not appended to Changed:\n%s", out)
	}
}

func TestEmptyChangedHeading(t *testing.T) {
	out, _ := run(t, head+"\n### Changed\n\n### Fixed\n\n- Bug.\n"+released)
	if !strings.Contains(out, "### Changed\n\n- Base image is `b:2`.\n\n### Fixed") {
		t.Errorf("bullet not under empty Changed:\n%s", out)
	}
}

func TestChangedGoesBeforeLaterTypes(t *testing.T) {
	out, version := run(t, head+"\n### Added\n\n- New.\n\n### Fixed\n\n- Bug.\n"+released)
	if version != "" {
		t.Errorf("version = %q, want none", version)
	}
	if !strings.Contains(out, "- New.\n\n### Changed\n\n- Base image is `b:2`.\n\n### Fixed") {
		t.Errorf("Changed not placed before Fixed:\n%s", out)
	}
}

func TestChangedGoesAfterEarlierTypes(t *testing.T) {
	out, _ := run(t, head+"\n### Added\n\n- New.\n"+released)
	if !strings.Contains(out, "- New.\n\n### Changed\n\n- Base image is `b:2`.\n\n## [0.1.0]") {
		t.Errorf("Changed not placed after Added:\n%s", out)
	}
}

func TestUnreleasedAlone(t *testing.T) {
	out, version := run(t, head)
	if version != "0.1.1" {
		t.Errorf("version = %q, want 0.1.1", version)
	}
	if !strings.HasSuffix(out, "### Changed\n\n- Base image is `b:2`.\n") {
		t.Errorf("entry not appended:\n%s", out)
	}
}

func TestMissingUnreleasedFails(t *testing.T) {
	if _, _, err := record([]byte("# Changelog\n"+released), "0.1.0", entry, "2026-11-03"); err == nil {
		t.Error("want an error without ## [Unreleased]")
	}
}

func TestNonSemverCurrentFails(t *testing.T) {
	if _, _, err := record([]byte(head), "13.20261027", entry, "2026-11-03"); err == nil {
		t.Error("want an error for a two-part version")
	}
}

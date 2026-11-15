package version

import (
	"regexp"
	"testing"
)

func TestVersionIsSemVer(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) {
		t.Fatalf("Version %q is not MAJOR.MINOR.PATCH", Version)
	}
}

func TestString(t *testing.T) {
	oldCommit, oldTime := Commit, BuildTime
	t.Cleanup(func() { Commit, BuildTime = oldCommit, oldTime })
	Commit, BuildTime = "abc1234", "2026-11-15T10:00:00Z"

	want := Version + " (commit abc1234, built 2026-11-15T10:00:00Z)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

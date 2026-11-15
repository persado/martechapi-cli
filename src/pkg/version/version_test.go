package version

import (
	"regexp"
	"testing"
)

func stamp(t *testing.T, commit, buildTime string) {
	t.Helper()
	oldCommit, oldTime := Commit, BuildTime
	t.Cleanup(func() { Commit, BuildTime = oldCommit, oldTime })
	Commit, BuildTime = commit, buildTime
}

func TestVersionIsSemVer(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) {
		t.Fatalf("Version %q is not MAJOR.MINOR.PATCH", Version)
	}
}

func TestString(t *testing.T) {
	stamp(t, "abc1234", "2026-11-15T10:00:00Z")
	want := Version + " (commit abc1234, built 2026-11-15T10:00:00Z)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestShort(t *testing.T) {
	tests := []struct{ commit, want string }{
		{"abc1234", Version + "-abc1234"},
		{"unknown", Version},
	}
	for _, tc := range tests {
		stamp(t, tc.commit, "unknown")
		if got := Short(); got != tc.want {
			t.Errorf("Short() with commit %q = %q, want %q", tc.commit, got, tc.want)
		}
	}
}

func TestBanner(t *testing.T) {
	stamp(t, "abc1234", "unknown")
	if got, want := Banner(), "Persado Martech API "+Version+"-abc1234"; got != want {
		t.Fatalf("Banner() = %q, want %q", got, want)
	}
}

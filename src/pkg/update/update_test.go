package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func redirectServer(t *testing.T, status int, location string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD", r.Method)
		}
		if location != "" {
			w.Header().Set("Location", location)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestLatest(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		location string
		want     string
		wantErr  bool
	}{
		{"tag redirect", http.StatusFound, "https://github.com/persado/martechapi-cli/releases/tag/v0.1.2", "0.1.2", false},
		{"no redirect", http.StatusOK, "", "", true},
		{"redirect without tag", http.StatusFound, "https://github.com/persado/martechapi-cli/releases", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Latest(context.Background(), http.DefaultClient, redirectServer(t, tc.status, tc.location))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("Latest() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLatestUnreachable(t *testing.T) {
	if _, err := Latest(context.Background(), http.DefaultClient, "http://127.0.0.1:1"); err == nil {
		t.Fatal("want an error for an unreachable server")
	}
}

func TestLatestBadURL(t *testing.T) {
	if _, err := Latest(context.Background(), http.DefaultClient, "://bad"); err == nil {
		t.Fatal("want an error for an invalid URL")
	}
}

func TestNewer(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"0.1.1", "0.1.2", true},
		{"0.1.9", "0.2.0", true},
		{"0.9.0", "1.0.0", true},
		{"0.1.2", "0.1.2", false},
		{"0.1.10", "0.1.9", false},
		{"0.1.1", "", false},
		{"0.1.1", "0.1", false},
		{"0.1.1", "0.1.x", false},
	}
	for _, tc := range tests {
		if got := Newer(tc.current, tc.latest); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestInstallCommand(t *testing.T) {
	if got := InstallCommand("windows"); got != InstallWindows {
		t.Errorf("windows: got %q", got)
	}
	for _, goos := range []string{"darwin", "linux"} {
		if got := InstallCommand(goos); got != InstallUnix {
			t.Errorf("%s: got %q", goos, got)
		}
	}
}

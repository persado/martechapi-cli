package api

import "testing"

func TestJobDone(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{JobRunning, false},
		{JobCompleted, true},
		{JobFailed, true},
		{"", false},
		{"queued", false},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			j := &Job{Status: tc.status}
			if got := j.Done(); got != tc.want {
				t.Errorf("Done() with status %q = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

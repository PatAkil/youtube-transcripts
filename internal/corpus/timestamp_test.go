package corpus

import (
	"testing"
	"time"
)

func TestTimestamp(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "[00:00]"},
		{999 * time.Millisecond, "[00:00]"},
		{12*time.Second + 340*time.Millisecond, "[00:12]"},
		{9*time.Minute + 5*time.Second, "[09:05]"},
		{59*time.Minute + 59*time.Second, "[59:59]"},
		{time.Hour, "[1:00:00]"},
		{2*time.Hour + 3*time.Minute + 4*time.Second, "[2:03:04]"},
		{-5 * time.Second, "[00:00]"},
	}
	for _, tt := range tests {
		if got := Timestamp(tt.in); got != tt.want {
			t.Errorf("Timestamp(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

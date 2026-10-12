package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFollowedChannels(t *testing.T) {
	corpusDir := t.TempDir()
	content := `[
  {"channel_id": "UCabc123", "name": "Example Channel"},
  {"channel_id": "UCdef456", "name": "Other Channel"}
]
`
	if err := os.WriteFile(filepath.Join(corpusDir, FollowedChannelsFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadFollowedChannels(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	want := []FollowedChannel{
		{ChannelID: "UCabc123", Name: "Example Channel"},
		{ChannelID: "UCdef456", Name: "Other Channel"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestReadFollowedChannelsRejectsMissingEmptyAndMalformedFile(t *testing.T) {
	tests := []struct {
		name    string
		content *string // nil: no file
		wantErr string
	}{
		{name: "missing", content: nil, wantErr: "followed-channels.json"},
		{name: "empty list", content: ptr("[]\n"), wantErr: "no Followed Channels"},
		{name: "blank file", content: ptr(""), wantErr: "followed-channels.json"},
		{name: "malformed", content: ptr("{not json"), wantErr: "followed-channels.json"},
		{name: "entry without channel id", content: ptr(`[{"name": "Nameless"}]`), wantErr: "channel_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corpusDir := t.TempDir()
			if tt.content != nil {
				if err := os.WriteFile(filepath.Join(corpusDir, FollowedChannelsFile), []byte(*tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := ReadFollowedChannels(corpusDir)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func ptr(s string) *string { return &s }

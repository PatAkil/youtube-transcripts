package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

var fixedNow = time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)

func manualTranscript() youtube.Transcript {
	return youtube.Transcript{
		Meta: youtube.VideoMeta{
			VideoID:     "dQw4w9WgXcQ",
			ChannelID:   "UCabc123",
			ChannelName: "Example Channel",
			Title:       "An example video",
			Published:   time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
		},
		Track: youtube.TrackManual,
		Cues: []youtube.Cue{
			{Start: 0, Text: "hello and welcome"},
			{Start: 12*time.Second + 340*time.Millisecond, Text: "this video is sponsored by Acme"},
			{Start: 61 * time.Minute, Text: "see you next time"},
		},
	}
}

func runFetch(t *testing.T, corpusDir string, fake *youtube.Fake, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	deps := Deps{YouTube: fake, Now: func() time.Time { return fixedNow }}
	code = Fetch(context.Background(), deps, corpusDir, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestFetchStoresTranscriptAndVideoMetadata(t *testing.T) {
	corpusDir := t.TempDir()
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}

	code, stdout, stderr := runFetch(t, corpusDir, fake, "https://youtu.be/dQw4w9WgXcQ?t=3")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	videoDir := filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ")

	wantTranscript := "[00:00] hello and welcome\n[00:12] this video is sponsored by Acme\n[1:01:00] see you next time\n"
	if got := readFile(t, filepath.Join(videoDir, "transcript.txt")); got != wantTranscript {
		t.Errorf("transcript.txt =\n%s\nwant\n%s", got, wantTranscript)
	}

	wantVideo := `{
  "video_id": "dQw4w9WgXcQ",
  "channel_id": "UCabc123",
  "channel_name": "Example Channel",
  "title": "An example video",
  "published": "2024-01-02T15:04:05Z",
  "fetched": "2026-10-12T09:30:00Z",
  "transcript": "ok",
  "track": "manual"
}
`
	if got := readFile(t, filepath.Join(videoDir, "video.json")); got != wantVideo {
		t.Errorf("video.json =\n%s\nwant\n%s", got, wantVideo)
	}

	wantLine := "dQw4w9WgXcQ  Example Channel  An example video  lines=3\n"
	if stdout != wantLine {
		t.Errorf("stdout = %q, want %q", stdout, wantLine)
	}
}

// decodeVideo is used by later tests that care about a few fields rather
// than the whole file.
func decodeVideo(t *testing.T, path string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &m); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return m
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(des))
	for _, de := range des {
		names = append(names, de.Name())
	}
	return names
}

var _ = strings.Join

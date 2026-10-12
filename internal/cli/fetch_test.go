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

func TestFetchRecordsAutoTrack(t *testing.T) {
	corpusDir := t.TempDir()
	tr := manualTranscript()
	tr.Track = youtube.TrackAuto
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": tr}}

	code, stdout, stderr := runFetch(t, corpusDir, fake, "dQw4w9WgXcQ")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	videoDir := filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ")
	video := decodeVideo(t, filepath.Join(videoDir, "video.json"))
	if video["track"] != "auto" || video["transcript"] != "ok" {
		t.Errorf("video.json track=%v transcript=%v, want auto/ok", video["track"], video["transcript"])
	}
	if !strings.Contains(readFile(t, filepath.Join(videoDir, "transcript.txt")), "[00:12] this video is sponsored by Acme") {
		t.Error("transcript.txt is missing the auto-track cues")
	}
	if !strings.HasSuffix(stdout, "lines=3\n") {
		t.Errorf("stdout = %q, want it to end with lines=3", stdout)
	}
}

func TestFetchWithNoEnglishTrackWritesVideoFileOnly(t *testing.T) {
	corpusDir := t.TempDir()
	tr := manualTranscript()
	tr.Track = youtube.TrackNone
	tr.Cues = nil
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": tr}}

	code, stdout, stderr := runFetch(t, corpusDir, fake, "dQw4w9WgXcQ")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	videoDir := filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ")
	if got := entries(t, videoDir); len(got) != 1 || got[0] != "video.json" {
		t.Errorf("video directory holds %v, want only video.json", got)
	}
	video := decodeVideo(t, filepath.Join(videoDir, "video.json"))
	if video["transcript"] != "none" || video["track"] != "" {
		t.Errorf("video.json transcript=%v track=%v, want none/\"\"", video["transcript"], video["track"])
	}
	wantLine := "dQw4w9WgXcQ  Example Channel  An example video  lines=0\n"
	if stdout != wantLine {
		t.Errorf("stdout = %q, want %q", stdout, wantLine)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFetchSkipsVideoAlreadyPresentWithoutCallingYouTube(t *testing.T) {
	corpusDir := t.TempDir()
	// The Video sits under a Channel directory other than the one the fake
	// would report, to show presence is decided by the video ID alone.
	videoDir := filepath.Join(corpusDir, "UCother", "dQw4w9WgXcQ")
	writeFile(t, filepath.Join(videoDir, "video.json"), "{}\n")
	writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), "[]\n")
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}

	code, stdout, stderr := runFetch(t, corpusDir, fake, "dQw4w9WgXcQ")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if stdout != "dQw4w9WgXcQ already present\n" {
		t.Errorf("stdout = %q, want %q", stdout, "dQw4w9WgXcQ already present\n")
	}
	if fake.Calls != 0 {
		t.Errorf("YouTube was called %d times, want 0", fake.Calls)
	}
	if got := readFile(t, filepath.Join(videoDir, "video.json")); got != "{}\n" {
		t.Errorf("video.json was rewritten to %q", got)
	}
	if _, err := os.Stat(filepath.Join(corpusDir, "UCabc123")); !os.IsNotExist(err) {
		t.Error("a new channel directory was created for an already present Video")
	}
}

func TestFetchForceRefetchesAndKeepsSegmentsFile(t *testing.T) {
	corpusDir := t.TempDir()
	videoDir := filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ")
	writeFile(t, filepath.Join(videoDir, "video.json"), "{\"stale\": true}\n")
	writeFile(t, filepath.Join(videoDir, "transcript.txt"), "[00:00] stale\n")
	segments := "{\n  \"processed\": \"2026-01-01T00:00:00Z\",\n  \"segments\": [\n    {\"sponsor\": \"Acme\", \"start\": 12.3, \"end\": 60, \"origin\": \"slot\", \"slot_index\": 0, \"opening_line\": \"hi\"}\n  ]\n}\n"
	writeFile(t, filepath.Join(videoDir, "segments.json"), segments)
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}

	code, stdout, stderr := runFetch(t, corpusDir, fake, "--force", "dQw4w9WgXcQ")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasPrefix(stdout, "dQw4w9WgXcQ  Example Channel  An example video  lines=3") {
		t.Errorf("stdout = %q, want the fetch summary line", stdout)
	}
	if got := readFile(t, filepath.Join(videoDir, "segments.json")); got != segments {
		t.Errorf("segments.json changed:\n%s", got)
	}
	if got := readFile(t, filepath.Join(videoDir, "transcript.txt")); strings.Contains(got, "stale") {
		t.Errorf("transcript.txt was not rewritten: %q", got)
	}
	if video := decodeVideo(t, filepath.Join(videoDir, "video.json")); video["title"] != "An example video" {
		t.Errorf("video.json was not rewritten: %v", video)
	}
	if got := entries(t, videoDir); strings.Join(got, ",") != "segments.json,transcript.txt,video.json" {
		t.Errorf("video directory holds %v", got)
	}
	if got := entries(t, filepath.Join(corpusDir, "UCabc123")); strings.Join(got, ",") != "dQw4w9WgXcQ" {
		t.Errorf("channel directory holds %v, want only the video directory (no temp or .old leftovers)", got)
	}
}

func TestFetchForceDoesNotCreateSegmentsFile(t *testing.T) {
	corpusDir := t.TempDir()
	videoDir := filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ")
	writeFile(t, filepath.Join(videoDir, "video.json"), "{}\n")
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}

	if code, _, stderr := runFetch(t, corpusDir, fake, "dQw4w9WgXcQ", "--force"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if got := entries(t, videoDir); strings.Join(got, ",") != "transcript.txt,video.json" {
		t.Errorf("video directory holds %v, want transcript.txt and video.json only", got)
	}
}

func TestFetchForceMovesVideoWhenChannelDirectoryChanged(t *testing.T) {
	corpusDir := t.TempDir()
	oldDir := filepath.Join(corpusDir, "UCother", "dQw4w9WgXcQ")
	writeFile(t, filepath.Join(oldDir, "video.json"), "{}\n")
	writeFile(t, filepath.Join(oldDir, "segments.json"), "{\"processed\": \"2026-01-01T00:00:00Z\", \"segments\": []}\n")
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}

	if code, _, stderr := runFetch(t, corpusDir, fake, "--force", "dQw4w9WgXcQ"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	newDir := filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ")
	if got := entries(t, newDir); strings.Join(got, ",") != "segments.json,transcript.txt,video.json" {
		t.Errorf("new video directory holds %v", got)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("stale video directory under the previous channel directory still exists")
	}
}

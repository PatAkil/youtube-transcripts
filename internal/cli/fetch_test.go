package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/sponsorblock"
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

// runFetch drives Fetch with a YouTube fake and a SponsorBlock fake that has
// no data on any Video.
func runFetch(t *testing.T, corpusDir string, fake *youtube.Fake, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return runFetchWithSlots(t, corpusDir, fake, &sponsorblock.Fake{}, args...)
}

func runFetchWithSlots(t *testing.T, corpusDir string, yt *youtube.Fake, sb *sponsorblock.Fake, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	deps := Deps{YouTube: yt, SponsorBlock: sb, Now: func() time.Time { return fixedNow }}
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

	// The SponsorBlock fake has no data on this Video: an empty list, not an error.
	if got := readFile(t, filepath.Join(videoDir, "slots.json")); got != "{\n  \"slots\": []\n}\n" {
		t.Errorf("slots.json = %q, want an empty slots list", got)
	}

	wantLine := "dQw4w9WgXcQ  Example Channel  An example video  lines=3  slots=0\n"
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
	if !strings.Contains(stdout, "lines=3") {
		t.Errorf("stdout = %q, want it to report lines=3", stdout)
	}
}

func TestFetchWithNoEnglishTrackWritesVideoAndSlotsFilesOnly(t *testing.T) {
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
	if got := entries(t, videoDir); strings.Join(got, ",") != "slots.json,video.json" {
		t.Errorf("video directory holds %v, want slots.json and video.json only", got)
	}
	video := decodeVideo(t, filepath.Join(videoDir, "video.json"))
	if video["transcript"] != "none" || video["track"] != "" {
		t.Errorf("video.json transcript=%v track=%v, want none/\"\"", video["transcript"], video["track"])
	}
	wantLine := "dQw4w9WgXcQ  Example Channel  An example video  lines=0  slots=0\n"
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
	// Any YouTube call for this Video fails, so reaching YouTube would show
	// up as a failed command rather than a silent refetch.
	fake := &youtube.Fake{Errors: map[string]error{"dQw4w9WgXcQ": errors.New("youtube must not be contacted for a present Video")}}

	code, stdout, stderr := runFetch(t, corpusDir, fake, "dQw4w9WgXcQ")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if stdout != "dQw4w9WgXcQ already present\n" {
		t.Errorf("stdout = %q, want %q", stdout, "dQw4w9WgXcQ already present\n")
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
	if got := entries(t, videoDir); strings.Join(got, ",") != "segments.json,slots.json,transcript.txt,video.json" {
		t.Errorf("video directory holds %v", got)
	}
	if got := entries(t, filepath.Join(corpusDir, "UCabc123")); strings.Join(got, ",") != "dQw4w9WgXcQ" {
		t.Errorf("channel directory holds %v, want only the video directory (no temp or .old leftovers)", got)
	}
}

func TestFetchForceSurvivesStaleRetiredDirectoryAndLeavesOnlyTheVideo(t *testing.T) {
	corpusDir := t.TempDir()
	channelDir := filepath.Join(corpusDir, "UCabc123")
	videoDir := filepath.Join(channelDir, "dQw4w9WgXcQ")
	writeFile(t, filepath.Join(videoDir, "video.json"), "{\"stale\": true}\n")
	// A retired copy left behind by an earlier --force that crashed between
	// its two renames. It is dot-prefixed so overview and presence skip it,
	// and it must not block the next --force.
	writeFile(t, filepath.Join(channelDir, ".dQw4w9WgXcQ.old", "video.json"), "{\"older\": true}\n")
	fake := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}

	code, stdout, stderr := runFetch(t, corpusDir, fake, "--force", "dQw4w9WgXcQ")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasPrefix(stdout, "dQw4w9WgXcQ  Example Channel  An example video  lines=3") {
		t.Errorf("stdout = %q, want the fetch summary line", stdout)
	}
	if video := decodeVideo(t, filepath.Join(videoDir, "video.json")); video["title"] != "An example video" {
		t.Errorf("video.json was not rewritten: %v", video)
	}
	if got := entries(t, channelDir); strings.Join(got, ",") != "dQw4w9WgXcQ" {
		t.Errorf("channel directory holds %v, want only the video directory (no retired or temp leftovers)", got)
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
	if got := entries(t, videoDir); strings.Join(got, ",") != "slots.json,transcript.txt,video.json" {
		t.Errorf("video directory holds %v, want slots.json, transcript.txt and video.json only", got)
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
	if got := entries(t, newDir); strings.Join(got, ",") != "segments.json,slots.json,transcript.txt,video.json" {
		t.Errorf("new video directory holds %v", got)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("stale video directory under the previous channel directory still exists")
	}
}

func TestFetchUpstreamErrorLeavesNothingBehind(t *testing.T) {
	corpusDir := t.TempDir()
	fake := &youtube.Fake{Errors: map[string]error{"dQw4w9WgXcQ": errors.New("youtube: page has no player response (consent or bot check)")}}

	code, stdout, stderr := runFetch(t, corpusDir, fake, "dQw4w9WgXcQ")

	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
	if !strings.Contains(stderr, "dQw4w9WgXcQ") || !strings.Contains(stderr, "no player response") {
		t.Errorf("stderr = %q, want the video ID and the upstream reason", stderr)
	}
	if got := entries(t, corpusDir); len(got) != 0 {
		t.Errorf("corpus holds %v after a failed fetch, want nothing", got)
	}
}

func TestFetchUpstreamErrorLeavesExistingChannelDirectoryUntouched(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "UCabc123", "otherVideo1", "video.json"), "{}\n")
	fake := &youtube.Fake{Errors: map[string]error{"dQw4w9WgXcQ": errors.New("boom")}}

	if code, _, _ := runFetch(t, corpusDir, fake, "dQw4w9WgXcQ"); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if got := entries(t, filepath.Join(corpusDir, "UCabc123")); strings.Join(got, ",") != "otherVideo1" {
		t.Errorf("channel directory holds %v, want only the pre-existing video", got)
	}
}

func TestFetchRejectsBadVideoReference(t *testing.T) {
	// The empty fake fails every call with "fake youtube: unknown video", so
	// that phrase on stderr would mean YouTube was contacted anyway.
	fake := &youtube.Fake{}
	code, _, stderr := runFetch(t, t.TempDir(), fake, "https://vimeo.com/12345")
	if code != 1 || stderr == "" {
		t.Errorf("exit %d, stderr %q; want 1 and a reason", code, stderr)
	}
	if strings.Contains(stderr, "fake youtube") {
		t.Errorf("YouTube was contacted for an unparseable reference: %q", stderr)
	}
}

func TestFetchWritesMergedSlotsFromSponsorBlock(t *testing.T) {
	corpusDir := t.TempDir()
	yt := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}
	sb := &sponsorblock.Fake{Windows: map[string][]sponsorblock.Window{"dQw4w9WgXcQ": {
		{Start: 45, End: 87, ID: "uuid-b"},
		{Start: 12.5, End: 60, ID: "uuid-a"},
		{Start: 300, End: 330.25, ID: "uuid-c"},
	}}}

	code, stdout, stderr := runFetchWithSlots(t, corpusDir, yt, sb, "dQw4w9WgXcQ")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	wantSlots := `{
  "slots": [
    {
      "start": 12.5,
      "end": 87,
      "source_ids": [
        "uuid-a",
        "uuid-b"
      ]
    },
    {
      "start": 300,
      "end": 330.25,
      "source_ids": [
        "uuid-c"
      ]
    }
  ]
}
`
	if got := readFile(t, filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ", "slots.json")); got != wantSlots {
		t.Errorf("slots.json =\n%s\nwant\n%s", got, wantSlots)
	}
	wantLine := "dQw4w9WgXcQ  Example Channel  An example video  lines=3  slots=2\n"
	if stdout != wantLine {
		t.Errorf("stdout = %q, want %q", stdout, wantLine)
	}
}

func TestFetchSponsorBlockErrorFailsAndLeavesNothingBehind(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "UCabc123", "otherVideo1", "video.json"), "{}\n")
	yt := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}
	sb := &sponsorblock.Fake{Errors: map[string]error{"dQw4w9WgXcQ": errors.New("sponsorblock: dQw4w9WgXcQ: HTTP 503")}}

	code, stdout, stderr := runFetchWithSlots(t, corpusDir, yt, sb, "dQw4w9WgXcQ")

	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
	if !strings.Contains(stderr, "dQw4w9WgXcQ") || !strings.Contains(stderr, "HTTP 503") {
		t.Errorf("stderr = %q, want the video ID and the SponsorBlock reason", stderr)
	}
	if got := entries(t, filepath.Join(corpusDir, "UCabc123")); strings.Join(got, ",") != "otherVideo1" {
		t.Errorf("channel directory holds %v, want only the pre-existing video (no partial Video directory)", got)
	}
	if got := entries(t, corpusDir); strings.Join(got, ",") != "UCabc123" {
		t.Errorf("corpus holds %v, want only the pre-existing channel directory", got)
	}
}

func TestFetchForceRewritesSlotsAndLeavesSegmentsByteIdentical(t *testing.T) {
	corpusDir := t.TempDir()
	videoDir := filepath.Join(corpusDir, "UCabc123", "dQw4w9WgXcQ")
	writeFile(t, filepath.Join(videoDir, "video.json"), "{}\n")
	writeFile(t, filepath.Join(videoDir, "slots.json"), "{\n  \"slots\": [\n    {\"start\": 1, \"end\": 2, \"source_ids\": [\"stale\"]}\n  ]\n}\n")
	// Deliberately not in the CLI's own formatting, so any rewrite would show.
	segments := "{\"processed\":\"2026-01-01T00:00:00Z\",\"segments\":[{\"sponsor\":\"Acme\",\"start\":12.5,\"end\":60,\"origin\":\"slot\",\"slot_index\":0,\"opening_line\":\"hi\"}]}"
	writeFile(t, filepath.Join(videoDir, "segments.json"), segments)
	yt := &youtube.Fake{Transcripts: map[string]youtube.Transcript{"dQw4w9WgXcQ": manualTranscript()}}
	sb := &sponsorblock.Fake{Windows: map[string][]sponsorblock.Window{"dQw4w9WgXcQ": {{Start: 12.5, End: 60, ID: "uuid-a"}}}}

	code, stdout, stderr := runFetchWithSlots(t, corpusDir, yt, sb, "--force", "dQw4w9WgXcQ")

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasSuffix(stdout, "lines=3  slots=1\n") {
		t.Errorf("stdout = %q, want it to end with slots=1", stdout)
	}
	wantSlots := "{\n  \"slots\": [\n    {\n      \"start\": 12.5,\n      \"end\": 60,\n      \"source_ids\": [\n        \"uuid-a\"\n      ]\n    }\n  ]\n}\n"
	if got := readFile(t, filepath.Join(videoDir, "slots.json")); got != wantSlots {
		t.Errorf("slots.json was not rewritten:\n%s", got)
	}
	if got, err := os.ReadFile(filepath.Join(videoDir, "segments.json")); err != nil || !bytes.Equal(got, []byte(segments)) {
		t.Errorf("segments.json changed (err %v):\n%s", err, got)
	}
}

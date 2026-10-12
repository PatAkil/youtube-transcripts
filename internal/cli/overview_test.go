package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/corpus"
)

func runOverview(t *testing.T, corpusDir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	deps := Deps{Now: func() time.Time { return fixedNow }}
	code = Overview(context.Background(), deps, corpusDir, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestOverviewHelp(t *testing.T) {
	code, stdout, stderr := runOverview(t, t.TempDir(), "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{"overview", "Sponsor", "overview.md", "segments.json", "--help"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("overview help lacks %q:\n%s", want, stdout)
		}
	}
}

// video returns hand-written Video metadata for the overview fixtures.
func video(channelID, channelName, videoID, title, published string) corpus.Video {
	pub, err := time.Parse(time.RFC3339, published)
	if err != nil {
		panic(err)
	}
	return corpus.Video{
		VideoID:     videoID,
		ChannelID:   channelID,
		ChannelName: channelName,
		Title:       title,
		Published:   pub,
		Fetched:     fixedNow,
		Transcript:  corpus.TranscriptOK,
		Track:       "manual",
	}
}

func writeVideoFixture(t *testing.T, corpusDir string, v corpus.Video) string {
	t.Helper()
	dir := corpus.VideoDir(corpusDir, v.ChannelID, v.VideoID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := corpus.MarshalJSON(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, corpus.VideoFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeSegmentsFixture(t *testing.T, videoDir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(videoDir, corpus.SegmentsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// segment renders one Segment object for a hand-written segments.json.
func segment(sponsor string, start, end float64) string {
	return fmt.Sprintf(`{"sponsor": %q, "start": %v, "end": %v, "origin": "transcript", "opening_line": "..."}`, sponsor, start, end)
}

func processed(segments ...string) string {
	return `{"processed": "2026-10-12T09:30:00Z", "segments": [` + strings.Join(segments, ", ") + "]}\n"
}

func TestOverviewWritesOneSponsorOnOneChannel(t *testing.T) {
	corpusDir := t.TempDir()
	dir := writeVideoFixture(t, corpusDir, video("UCabc123", "Example Channel", "dQw4w9WgXcQ", "An example video", "2024-01-02T15:04:05Z"))
	writeSegmentsFixture(t, dir, processed(segment("Acme", 12.5, 60)))

	code, stdout, stderr := runOverview(t, corpusDir)

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want := `# Sponsorship overview

## By Sponsor

### Acme

- Example Channel (UCabc123)
  - An example video (dQw4w9WgXcQ): [[00:12]](https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=12s)

## By Channel

### Example Channel (UCabc123)

- Acme: 1 Segment

## Awaiting agent pass

_None._

## No transcript

_None._
`
	if got := readFile(t, filepath.Join(corpusDir, "overview.md")); got != want {
		t.Errorf("overview.md =\n%s\nwant\n%s", got, want)
	}
	if wantOut := "wrote " + filepath.Join(corpusDir, "overview.md") + "  sponsors=1  channels=1  awaiting=0\n"; stdout != wantOut {
		t.Errorf("stdout = %q, want %q", stdout, wantOut)
	}
}

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

func TestOverviewGroupsAcrossChannelsInStableOrder(t *testing.T) {
	corpusDir := t.TempDir()
	// Directory names and file order are deliberately out of display order.
	zed := writeVideoFixture(t, corpusDir, video("UCzzz", "Zed Talks", "zzzzzzzzzz1", "Newer on Zed", "2025-03-01T00:00:00Z"))
	writeSegmentsFixture(t, zed, processed(segment("Globex", 30, 90), segment("Acme", 400, 460)))
	zedOld := writeVideoFixture(t, corpusDir, video("UCzzz", "Zed Talks", "zzzzzzzzzz0", "Older on Zed", "2024-06-01T00:00:00Z"))
	writeSegmentsFixture(t, zedOld, processed(segment("Acme", 65, 120)))
	alpha := writeVideoFixture(t, corpusDir, video("UCaaa", "alpha channel", "aaaaaaaaaa1", "Alpha one", "2024-01-02T15:04:05Z"))
	writeSegmentsFixture(t, alpha, processed(segment("Acme", 12, 60)))

	code, stdout, stderr := runOverview(t, corpusDir)

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want := `# Sponsorship overview

## By Sponsor

### Acme

- alpha channel (UCaaa)
  - Alpha one (aaaaaaaaaa1): [[00:12]](https://www.youtube.com/watch?v=aaaaaaaaaa1&t=12s)
- Zed Talks (UCzzz)
  - Older on Zed (zzzzzzzzzz0): [[01:05]](https://www.youtube.com/watch?v=zzzzzzzzzz0&t=65s)
  - Newer on Zed (zzzzzzzzzz1): [[06:40]](https://www.youtube.com/watch?v=zzzzzzzzzz1&t=400s)

### Globex

- Zed Talks (UCzzz)
  - Newer on Zed (zzzzzzzzzz1): [[00:30]](https://www.youtube.com/watch?v=zzzzzzzzzz1&t=30s)

## By Channel

### alpha channel (UCaaa)

- Acme: 1 Segment

### Zed Talks (UCzzz)

- Acme: 2 Segments
- Globex: 1 Segment

## Awaiting agent pass

_None._

## No transcript

_None._
`
	if got := readFile(t, filepath.Join(corpusDir, "overview.md")); got != want {
		t.Errorf("overview.md =\n%s\nwant\n%s", got, want)
	}
	if !strings.HasSuffix(stdout, "sponsors=2  channels=2  awaiting=0\n") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestOverviewMergesSponsorSpellingsCaseInsensitively(t *testing.T) {
	corpusDir := t.TempDir()
	// "NordVPN" twice, " Nordvpn " once, "nordvpn" once (and last in walk
	// order): one Sponsor, shown as NordVPN.
	a := writeVideoFixture(t, corpusDir, video("UCaaa", "Alpha", "aaaaaaaaaa1", "A1", "2024-01-01T00:00:00Z"))
	writeSegmentsFixture(t, a, processed(segment("NordVPN", 10, 70)))
	b := writeVideoFixture(t, corpusDir, video("UCaaa", "Alpha", "aaaaaaaaaa2", "A2", "2024-02-01T00:00:00Z"))
	writeSegmentsFixture(t, b, processed(segment("NordVPN", 20, 80)))
	c := writeVideoFixture(t, corpusDir, video("UCbbb", "Beta", "bbbbbbbbbb1", "B1", "2024-03-01T00:00:00Z"))
	writeSegmentsFixture(t, c, processed(segment(" Nordvpn ", 30, 90), segment("nordvpn", 500, 560)))
	// "Squarespace" and "squarespace" once each: a tie, so the alphabetically
	// first spelling wins even though it is seen first.
	d := writeVideoFixture(t, corpusDir, video("UCbbb", "Beta", "bbbbbbbbbb2", "B2", "2024-04-01T00:00:00Z"))
	writeSegmentsFixture(t, d, processed(segment("Squarespace", 5, 50), segment("squarespace", 700, 750)))

	code, stdout, stderr := runOverview(t, corpusDir)

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	got := readFile(t, filepath.Join(corpusDir, "overview.md"))
	bySponsor := got[:strings.Index(got, "## By Channel")]
	if n := strings.Count(bySponsor, "\n### "); n != 2 {
		t.Errorf("By Sponsor has %d Sponsors, want 2:\n%s", n, bySponsor)
	}
	for _, want := range []string{"\n### NordVPN\n", "\n### Squarespace\n", "- NordVPN: 2 Segments\n", "- Squarespace: 2 Segments\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("overview.md lacks %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"nordvpn", "Nordvpn", "squarespace"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("overview.md shows non-canonical spelling %q:\n%s", unwanted, got)
		}
	}
	if !strings.HasSuffix(stdout, "sponsors=2  channels=2  awaiting=0\n") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestOverviewCountsTwoReadsForOneSponsorInOneVideoAsTwoSegments(t *testing.T) {
	corpusDir := t.TempDir()
	dir := writeVideoFixture(t, corpusDir, video("UCabc123", "Example Channel", "dQw4w9WgXcQ", "An example video", "2024-01-02T15:04:05Z"))
	// Mid-roll read written first; the overview orders starts ascending.
	writeSegmentsFixture(t, dir, processed(segment("Acme", 1830, 1890), segment("Acme", 12.5, 60)))

	code, _, stderr := runOverview(t, corpusDir)

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	got := readFile(t, filepath.Join(corpusDir, "overview.md"))
	for _, want := range []string{
		"  - An example video (dQw4w9WgXcQ): [[00:12]](https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=12s), [[30:30]](https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=1830s)\n",
		"- Acme: 2 Segments\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("overview.md lacks %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "An example video (dQw4w9WgXcQ)"); n != 1 {
		t.Errorf("Video listed %d times under its Sponsor, want once with both start times:\n%s", n, got)
	}
}

func TestOverviewListsBacklogAndNoTranscriptSeparatelyFromProcessedNoneFound(t *testing.T) {
	corpusDir := t.TempDir()
	// Processed, no Sponsors found: appears nowhere.
	none := writeVideoFixture(t, corpusDir, video("UCaaa", "Alpha", "aaaaaaaaaa1", "Nothing sponsored", "2024-01-01T00:00:00Z"))
	writeSegmentsFixture(t, none, processed())
	// No segments.json yet: backlog, by Channel, by publish date.
	writeVideoFixture(t, corpusDir, video("UCaaa", "Alpha", "aaaaaaaaaa3", "Later unprocessed", "2024-03-01T00:00:00Z"))
	writeVideoFixture(t, corpusDir, video("UCaaa", "Alpha", "aaaaaaaaaa2", "Earlier unprocessed", "2024-02-01T00:00:00Z"))
	writeVideoFixture(t, corpusDir, video("UCbbb", "Beta", "bbbbbbbbbb1", "Beta unprocessed", "2024-01-15T00:00:00Z"))
	// No English track: listed under No transcript, not in the backlog.
	silent := video("UCbbb", "Beta", "bbbbbbbbbb2", "Silent", "2024-04-01T00:00:00Z")
	silent.Transcript = corpus.TranscriptNone
	silent.Track = ""
	writeVideoFixture(t, corpusDir, silent)

	code, stdout, stderr := runOverview(t, corpusDir)

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want := `# Sponsorship overview

## By Sponsor

_None._

## By Channel

_None._

## Awaiting agent pass

### Alpha (UCaaa)

- Earlier unprocessed (aaaaaaaaaa2)
- Later unprocessed (aaaaaaaaaa3)

### Beta (UCbbb)

- Beta unprocessed (bbbbbbbbbb1)

## No transcript

- Beta (UCbbb): Silent (bbbbbbbbbb2)
`
	if got := readFile(t, filepath.Join(corpusDir, "overview.md")); got != want {
		t.Errorf("overview.md =\n%s\nwant\n%s", got, want)
	}
	// channels= counts the Channels listed under By Channel, so it matches
	// the section: none here, since no Channel has a Segment yet.
	if !strings.HasSuffix(stdout, "sponsors=0  channels=0  awaiting=3\n") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestOverviewNamesUnparseableFilesOnStderrAndSkipsThem(t *testing.T) {
	corpusDir := t.TempDir()
	good := writeVideoFixture(t, corpusDir, video("UCaaa", "Alpha", "aaaaaaaaaa1", "Good", "2024-01-01T00:00:00Z"))
	writeSegmentsFixture(t, good, processed(segment("Acme", 12, 60)))
	badSegments := writeVideoFixture(t, corpusDir, video("UCaaa", "Alpha", "aaaaaaaaaa2", "Hand-edited badly", "2024-02-01T00:00:00Z"))
	writeSegmentsFixture(t, badSegments, `{"processed": "2026-10-12T09:30:00Z", "segments": [{"sponsor": "Globex", "start": 5, "end": 50, "origin": "guess"}]`+"\n")
	badVideo := writeVideoFixture(t, corpusDir, video("UCbbb", "Beta", "bbbbbbbbbb1", "Broken metadata", "2024-03-01T00:00:00Z"))
	if err := os.WriteFile(filepath.Join(badVideo, corpus.VideoFile), []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runOverview(t, corpusDir)

	if code != 0 {
		t.Errorf("exit %d, want 0 even with skipped files", code)
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stderr has %d lines, want one per skipped file:\n%s", len(lines), stderr)
	}
	if !strings.Contains(lines[0], filepath.Join(badSegments, corpus.SegmentsFile)) {
		t.Errorf("first stderr line %q does not name the bad segments.json", lines[0])
	}
	if !strings.Contains(lines[1], filepath.Join(badVideo, corpus.VideoFile)) {
		t.Errorf("second stderr line %q does not name the bad video.json", lines[1])
	}
	got := readFile(t, filepath.Join(corpusDir, "overview.md"))
	if !strings.Contains(got, "### Acme\n") || !strings.Contains(got, "Good (aaaaaaaaaa1)") {
		t.Errorf("overview.md lost the good Video:\n%s", got)
	}
	for _, unwanted := range []string{"Globex", "aaaaaaaaaa2", "bbbbbbbbbb1", "Beta"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("overview.md mentions skipped %q:\n%s", unwanted, got)
		}
	}
	if !strings.HasSuffix(stdout, "sponsors=1  channels=1  awaiting=0\n") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestOverviewIsByteIdenticalOnAnUnchangedCorpus(t *testing.T) {
	corpusDir := t.TempDir()
	a := writeVideoFixture(t, corpusDir, video("UCaaa", "Alpha", "aaaaaaaaaa1", "A1", "2024-01-01T00:00:00Z"))
	writeSegmentsFixture(t, a, processed(segment("Acme", 12, 60), segment("Globex", 700, 760)))
	b := writeVideoFixture(t, corpusDir, video("UCbbb", "Beta", "bbbbbbbbbb1", "B1", "2024-02-01T00:00:00Z"))
	writeSegmentsFixture(t, b, processed(segment("acme", 30, 90)))
	writeVideoFixture(t, corpusDir, video("UCbbb", "Beta", "bbbbbbbbbb2", "B2", "2024-03-01T00:00:00Z"))

	if code, _, stderr := runOverview(t, corpusDir); code != 0 || stderr != "" {
		t.Fatalf("first run: exit %d, stderr %q", code, stderr)
	}
	first := readFile(t, filepath.Join(corpusDir, "overview.md"))
	later := Deps{Now: func() time.Time { return fixedNow.Add(48 * time.Hour) }}
	var out, errOut bytes.Buffer
	if code := Overview(context.Background(), later, corpusDir, nil, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("second run: exit %d, stderr %q", code, errOut.String())
	}
	second := readFile(t, filepath.Join(corpusDir, "overview.md"))
	if first != second {
		t.Errorf("overview.md changed between runs:\n%s\n---\n%s", first, second)
	}
	if strings.Contains(first, "2026") {
		t.Errorf("overview.md carries a generation timestamp:\n%s", first)
	}
}

func TestOverviewRejectsUnexpectedArguments(t *testing.T) {
	corpusDir := t.TempDir()
	code, _, stderr := runOverview(t, corpusDir, "dQw4w9WgXcQ")
	if code != 1 || !strings.Contains(stderr, "dQw4w9WgXcQ") || !strings.Contains(stderr, "Usage:") {
		t.Errorf("exit %d, stderr %q; want 1 naming the argument and the usage", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(corpusDir, "overview.md")); !os.IsNotExist(err) {
		t.Errorf("overview.md written despite the bad argument (stat err %v)", err)
	}
}

func TestOverviewOnEmptyOrMissingCorpusWritesAnEmptyOverview(t *testing.T) {
	corpusDir := filepath.Join(t.TempDir(), "corpus")
	code, stdout, stderr := runOverview(t, corpusDir)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	got := readFile(t, filepath.Join(corpusDir, "overview.md"))
	if strings.Count(got, "_None._") != 4 {
		t.Errorf("empty overview should mark all four sections as none:\n%s", got)
	}
	if !strings.HasSuffix(stdout, "sponsors=0  channels=0  awaiting=0\n") {
		t.Errorf("stdout = %q", stdout)
	}
}

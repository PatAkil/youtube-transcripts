package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/sponsorblock"
	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

// Two Followed Channels, each with two Videos in its feed.
const twoFollowedChannels = `[
  {"channel_id": "UCabc123", "name": "Example Channel"},
  {"channel_id": "UCdef456", "name": "Other Channel"}
]
`

func meta(videoID, channelID, channelName, title string) youtube.VideoMeta {
	return youtube.VideoMeta{
		VideoID:     videoID,
		ChannelID:   channelID,
		ChannelName: channelName,
		Title:       title,
		Published:   time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC),
	}
}

func transcriptFor(m youtube.VideoMeta) youtube.Transcript {
	return youtube.Transcript{
		Meta:  m,
		Track: youtube.TrackManual,
		Cues: []youtube.Cue{
			{Start: 0, Text: "hello and welcome"},
			{Start: 12 * time.Second, Text: "this video is sponsored by Acme"},
		},
	}
}

// twoChannelFake returns a Fake whose feeds list four Videos across two
// Channels and whose Transcripts cover all four.
func twoChannelFake() *youtube.Fake {
	videos := []youtube.VideoMeta{
		meta("aaaaaaaaaaa", "UCabc123", "Example Channel", "First example"),
		meta("bbbbbbbbbbb", "UCabc123", "Example Channel", "Second example"),
		meta("ccccccccccc", "UCdef456", "Other Channel", "First other"),
		meta("ddddddddddd", "UCdef456", "Other Channel", "Second other"),
	}
	fake := &youtube.Fake{
		Feeds:       map[string][]youtube.VideoMeta{},
		Transcripts: map[string]youtube.Transcript{},
		Errors:      map[string]error{},
	}
	for _, v := range videos {
		fake.Feeds[v.ChannelID] = append(fake.Feeds[v.ChannelID], v)
		fake.Transcripts[v.VideoID] = transcriptFor(v)
	}
	return fake
}

func runSync(t *testing.T, corpusDir string, fake *youtube.Fake, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	deps := Deps{YouTube: fake, SponsorBlock: &sponsorblock.Fake{}, Now: func() time.Time { return fixedNow }}
	code = Sync(context.Background(), deps, corpusDir, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestSyncFetchesNewVideosOfEveryFollowedChannel(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), twoFollowedChannels)
	fake := twoChannelFake()

	code, stdout, stderr := runSync(t, corpusDir, fake)

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{
		"UCabc123/aaaaaaaaaaa/video.json", "UCabc123/aaaaaaaaaaa/transcript.txt",
		"UCabc123/bbbbbbbbbbb/video.json",
		"UCdef456/ccccccccccc/video.json",
		"UCdef456/ddddddddddd/video.json", "UCdef456/ddddddddddd/transcript.txt",
	} {
		if _, err := os.Stat(filepath.Join(corpusDir, want)); err != nil {
			t.Errorf("%s not written: %v", want, err)
		}
	}
	video := decodeVideo(t, filepath.Join(corpusDir, "UCdef456", "ccccccccccc", "video.json"))
	if video["title"] != "First other" || video["channel_name"] != "Other Channel" || video["fetched"] != "2026-10-12T09:30:00Z" {
		t.Errorf("video.json = %v", video)
	}

	for _, want := range []string{
		"aaaaaaaaaaa  Example Channel  First example  lines=2  slots=0\n",
		"bbbbbbbbbbb  Example Channel  Second example  lines=2  slots=0\n",
		"ccccccccccc  Other Channel  First other  lines=2  slots=0\n",
		"ddddddddddd  Other Channel  Second other  lines=2  slots=0\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	assertSummary(t, stdout, 2, 4, 0, 0)
}

// assertSummary checks the closing summary's four counts.
func assertSummary(t *testing.T, stdout string, walked, fetched, skipped, failed int) {
	t.Helper()
	for _, want := range []string{
		"channels walked: " + strconv.Itoa(walked),
		"videos fetched: " + strconv.Itoa(fetched),
		"videos skipped (already present): " + strconv.Itoa(skipped),
		"videos failed: " + strconv.Itoa(failed),
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("summary lacks %q:\n%s", want, stdout)
		}
	}
}

func TestSyncSecondRunSkipsEverythingAlreadyPresent(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), twoFollowedChannels)
	fake := twoChannelFake()
	if code, _, stderr := runSync(t, corpusDir, fake); code != 0 {
		t.Fatalf("first run: exit %d, stderr %q", code, stderr)
	}
	// Mark the stored files so a refetch would be visible.
	marker := filepath.Join(corpusDir, "UCabc123", "aaaaaaaaaaa", "video.json")
	writeFile(t, marker, "{\"marker\": true}\n")
	fake.Calls = 0

	code, stdout, stderr := runSync(t, corpusDir, fake)

	if code != 0 || stderr != "" {
		t.Fatalf("second run: exit %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "lines=") {
		t.Errorf("second run printed a fetched Video:\n%s", stdout)
	}
	assertSummary(t, stdout, 2, 0, 4, 0)
	if got := readFile(t, marker); got != "{\"marker\": true}\n" {
		t.Errorf("already present Video was rewritten: %q", got)
	}
	// Only the two feed listings reach YouTube; no Transcript is refetched.
	if fake.Calls != 2 {
		t.Errorf("YouTube was called %d times on the second run, want 2 (one feed per Channel)", fake.Calls)
	}
}

func TestSyncContinuesPastChannelWhoseFeedFails(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), twoFollowedChannels)
	fake := twoChannelFake()
	fake.Errors["UCabc123"] = errors.New("HTTP 404")

	code, stdout, stderr := runSync(t, corpusDir, fake)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(corpusDir, "UCdef456", "ddddddddddd", "video.json")); err != nil {
		t.Errorf("other Channel's Video not fetched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpusDir, "UCabc123")); !os.IsNotExist(err) {
		t.Error("a directory was created for the Channel whose feed failed")
	}
	assertSummary(t, stdout, 1, 2, 0, 0)
	if !strings.Contains(stdout, "failed channels: Example Channel (UCabc123)") {
		t.Errorf("summary does not name the failed Channel:\n%s", stdout)
	}
	if !strings.Contains(stderr, "UCabc123") || !strings.Contains(stderr, "HTTP 404") {
		t.Errorf("stderr lacks the Channel and its feed error: %q", stderr)
	}
}

func TestSyncContinuesPastVideoWhoseFetchFails(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), twoFollowedChannels)
	fake := twoChannelFake()
	fake.Errors["bbbbbbbbbbb"] = errors.New("track download: HTTP 500")

	code, stdout, stderr := runSync(t, corpusDir, fake)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, id := range []string{"aaaaaaaaaaa", "ccccccccccc", "ddddddddddd"} {
		if _, err := corpusVideoDir(corpusDir, id); err != nil {
			t.Errorf("Video %s not fetched: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(corpusDir, "UCabc123", "bbbbbbbbbbb")); !os.IsNotExist(err) {
		t.Error("a directory was left behind for the Video whose fetch failed")
	}
	assertSummary(t, stdout, 2, 3, 0, 1)
	if !strings.Contains(stdout, "failed videos: bbbbbbbbbbb (Example Channel)") {
		t.Errorf("summary does not name the failed Video:\n%s", stdout)
	}
	if !strings.Contains(stderr, "bbbbbbbbbbb") || !strings.Contains(stderr, "HTTP 500") {
		t.Errorf("stderr lacks the Video and its error: %q", stderr)
	}
}

// corpusVideoDir finds the Video's directory under any Channel directory.
func corpusVideoDir(corpusDir, videoID string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(corpusDir, "*", videoID, "video.json"))
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("found %d video.json for %s, want 1", len(matches), videoID)
	}
	return filepath.Dir(matches[0]), nil
}

func TestSyncExitsOneWhenEveryChannelFails(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), twoFollowedChannels)
	fake := twoChannelFake()
	fake.Errors["UCabc123"] = errors.New("HTTP 404")
	fake.Errors["UCdef456"] = errors.New("HTTP 503")

	code, stdout, stderr := runSync(t, corpusDir, fake)

	if code != 1 {
		t.Errorf("exit %d, want 1 when every Channel failed", code)
	}
	assertSummary(t, stdout, 0, 0, 0, 0)
	if !strings.Contains(stdout, "failed channels: Example Channel (UCabc123), Other Channel (UCdef456)") {
		t.Errorf("summary does not name both failed Channels:\n%s", stdout)
	}
	if !strings.Contains(stderr, "HTTP 404") || !strings.Contains(stderr, "HTTP 503") {
		t.Errorf("stderr lacks the feed errors: %q", stderr)
	}
}

func TestSyncReportsMissingOrEmptyFollowedChannelsFile(t *testing.T) {
	tests := []struct {
		name    string
		content string // "" means no file
	}{
		{name: "missing file"},
		{name: "empty list", content: "[]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corpusDir := t.TempDir()
			if tt.content != "" {
				writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), tt.content)
			}
			fake := twoChannelFake()

			code, _, stderr := runSync(t, corpusDir, fake)

			if code != 1 {
				t.Errorf("exit %d, want 1", code)
			}
			if !strings.Contains(stderr, "followed-channels.json") {
				t.Errorf("stderr does not name the Followed Channels file: %q", stderr)
			}
			if fake.Calls != 0 {
				t.Errorf("YouTube was called %d times, want 0", fake.Calls)
			}
		})
	}
}

func TestSyncHelp(t *testing.T) {
	code, stdout, stderr := runSync(t, t.TempDir(), &youtube.Fake{}, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{"sync", "Followed Channel", "followed-channels.json", "already present", "--help"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("sync help lacks %q:\n%s", want, stdout)
		}
	}
}

func TestSyncRejectsUnexpectedArgument(t *testing.T) {
	code, _, stderr := runSync(t, t.TempDir(), &youtube.Fake{}, "UCabc123")
	if code != 1 || !strings.Contains(stderr, "UCabc123") {
		t.Errorf("exit %d, stderr %q; want 1 naming the argument", code, stderr)
	}
}

func TestSyncTreatsVideoUnderAnotherChannelDirectoryAsPresent(t *testing.T) {
	corpusDir := t.TempDir()
	writeFile(t, filepath.Join(corpusDir, "followed-channels.json"), twoFollowedChannels)
	// Video ccccccccccc belongs to UCdef456 in the feed, but it was fetched
	// by hand earlier and sits under another Channel directory.
	strayDir := filepath.Join(corpusDir, "UCstray", "ccccccccccc")
	writeFile(t, filepath.Join(strayDir, "video.json"), "{}\n")
	fake := twoChannelFake()

	code, stdout, stderr := runSync(t, corpusDir, fake)

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	assertSummary(t, stdout, 2, 3, 1, 0)
	if strings.Contains(stdout, "ccccccccccc  Other Channel") {
		t.Errorf("stray Video was refetched:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(corpusDir, "UCdef456", "ccccccccccc")); !os.IsNotExist(err) {
		t.Error("stray Video was fetched again under its feed Channel directory")
	}
	if got := readFile(t, filepath.Join(strayDir, "video.json")); got != "{}\n" {
		t.Errorf("stray Video's files were rewritten: %q", got)
	}
}

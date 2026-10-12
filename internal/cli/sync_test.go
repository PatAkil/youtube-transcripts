package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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
	deps := Deps{YouTube: fake, Now: func() time.Time { return fixedNow }}
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
		"aaaaaaaaaaa  Example Channel  First example  lines=2\n",
		"bbbbbbbbbbb  Example Channel  Second example  lines=2\n",
		"ccccccccccc  Other Channel  First other  lines=2\n",
		"ddddddddddd  Other Channel  Second other  lines=2\n",
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

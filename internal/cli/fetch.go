package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/PatAkil/youtube-transcripts/internal/corpus"
	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

const fetchUsage = `Store one Video's Transcript, metadata and Sponsor Slots in the Corpus.

Usage:
  youtube-transcripts [--corpus DIR] fetch [--force] <video ID or URL>

The Video may be given as a bare YouTube video ID or as a watch, youtu.be,
shorts or embed URL. The Transcript comes from the Video's English track
(a manual track is preferred over an auto-generated one) and is written as
timestamped text under <corpus>/<channel ID>/<video ID>/, next to video.json.
A Video with no English track is recorded with transcript "none". The
Sponsor Slots come from SponsorBlock's "sponsor" category, merged where they
overlap, and are written to slots.json in the same directory; a Video
SponsorBlock has no data on gets an empty Slots list.

A Video already in the Corpus is skipped unless --force is given, which
refetches it. A segments.json written by the agent pass is never touched.

Flags:
  --force    Refetch a Video that is already present
  --help     Show this help
`

// Fetch stores one Video's Transcript and metadata in the Corpus. It returns
// the process exit code.
func Fetch(ctx context.Context, deps Deps, corpusDir string, args []string, stdout, stderr io.Writer) int {
	var force bool
	var positional []string
	for _, a := range args {
		switch a {
		case "--force", "-force":
			force = true
		case "--help", "-h":
			fmt.Fprint(stdout, fetchUsage)
			return 0
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		fmt.Fprintf(stderr, "fetch: expected exactly one video ID or URL\n\n%s", fetchUsage)
		return 1
	}
	videoID, err := youtube.ParseVideoID(positional[0])
	if err != nil {
		fmt.Fprintf(stderr, "fetch: %v\n", err)
		return 1
	}

	res, err := fetchVideo(ctx, deps, corpusDir, videoID, force)
	if err != nil {
		fmt.Fprintf(stderr, "fetch %s: %v\n", videoID, err)
		return 1
	}
	if res.AlreadyPresent {
		fmt.Fprintf(stdout, "%s already present\n", videoID)
		return 0
	}
	fmt.Fprintln(stdout, res.Summary())
	return 0
}

// fetchResult is what one Video fetch produced. sync reuses it for its own
// per-Video reporting.
type fetchResult struct {
	// AlreadyPresent is set when the Video was in the Corpus and force was
	// false; nothing was fetched or written.
	AlreadyPresent bool
	Video          corpus.Video
	Lines          int
	Slots          int
}

// Summary is the one-line report printed after a successful fetch.
func (r fetchResult) Summary() string {
	return fmt.Sprintf("%s  %s  %s  lines=%d  slots=%d", r.Video.VideoID, r.Video.ChannelName, r.Video.Title, r.Lines, r.Slots)
}

// fetchVideo is the per-Video fetch shared by fetch and sync: presence check,
// upstream calls, atomic write of the Video directory. It prints nothing.
func fetchVideo(ctx context.Context, deps Deps, corpusDir, videoID string, force bool) (fetchResult, error) {
	existing, err := corpus.FindVideo(corpusDir, videoID)
	if err != nil {
		return fetchResult{}, err
	}
	if existing != "" && !force {
		return fetchResult{AlreadyPresent: true}, nil
	}

	tr, err := deps.YouTube.FetchTranscript(ctx, videoID)
	if err != nil {
		return fetchResult{}, err
	}
	windows, err := deps.SponsorBlock.FetchSlots(ctx, videoID)
	if err != nil {
		return fetchResult{}, err
	}
	slots := corpus.Slots{Slots: corpus.MergeWindows(windows)}

	video := corpus.Video{
		VideoID:     tr.Meta.VideoID,
		ChannelID:   tr.Meta.ChannelID,
		ChannelName: tr.Meta.ChannelName,
		Title:       tr.Meta.Title,
		Published:   tr.Meta.Published,
		Fetched:     deps.Now(),
		Transcript:  corpus.TranscriptOK,
		Track:       string(tr.Track),
	}
	if tr.Track == youtube.TrackNone {
		video.Transcript = corpus.TranscriptNone
	}
	videoJSON, err := corpus.MarshalJSON(video)
	if err != nil {
		return fetchResult{}, err
	}
	slotsJSON, err := corpus.MarshalJSON(slots)
	if err != nil {
		return fetchResult{}, err
	}
	files := map[string][]byte{corpus.VideoFile: videoJSON, corpus.SlotsFile: slotsJSON}
	if tr.Track != youtube.TrackNone {
		files[corpus.TranscriptFile] = []byte(renderTranscript(tr.Cues))
	}
	// The agent pass owns segments.json; a refetch carries it over untouched.
	keep := []string{corpus.SegmentsFile}
	if err := corpus.WriteVideoDir(corpusDir, video.ChannelID, video.VideoID, files, existing, keep); err != nil {
		return fetchResult{}, err
	}
	return fetchResult{Video: video, Lines: len(tr.Cues), Slots: len(slots.Slots)}, nil
}

func renderTranscript(cues []youtube.Cue) string {
	var b strings.Builder
	for _, c := range cues {
		b.WriteString(corpus.Timestamp(c.Start))
		b.WriteByte(' ')
		b.WriteString(c.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

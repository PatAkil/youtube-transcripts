package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/PatAkil/youtube-transcripts/internal/corpus"
	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

const fetchUsage = `Store one Video's Transcript and metadata in the Corpus.

Usage:
  youtube-transcripts [--corpus DIR] fetch [--force] <video ID or URL>

The Video may be given as a bare YouTube video ID or as a watch, youtu.be,
shorts or embed URL. The Transcript comes from the Video's English track
(a manual track is preferred over an auto-generated one) and is written as
timestamped text under <corpus>/<channel ID>/<video ID>/, next to video.json.
A Video with no English track is recorded with transcript "none".

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

	existing, err := corpus.FindVideo(corpusDir, videoID)
	if err != nil {
		fmt.Fprintf(stderr, "fetch %s: %v\n", videoID, err)
		return 1
	}
	if existing != "" && !force {
		fmt.Fprintf(stdout, "%s already present\n", videoID)
		return 0
	}

	tr, err := deps.YouTube.FetchTranscript(ctx, videoID)
	if err != nil {
		fmt.Fprintf(stderr, "fetch %s: %v\n", videoID, err)
		return 1
	}

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
		fmt.Fprintf(stderr, "fetch %s: %v\n", videoID, err)
		return 1
	}
	files := map[string][]byte{corpus.VideoFile: videoJSON}
	if tr.Track != youtube.TrackNone {
		files[corpus.TranscriptFile] = []byte(renderTranscript(tr.Cues))
	}
	// The agent pass owns segments.json; a refetch carries it over untouched.
	keep := []string{corpus.SegmentsFile}
	if err := corpus.WriteVideoDir(corpusDir, video.ChannelID, video.VideoID, files, existing, keep); err != nil {
		fmt.Fprintf(stderr, "fetch %s: %v\n", videoID, err)
		return 1
	}

	fmt.Fprintf(stdout, "%s  %s  %s  lines=%d\n", video.VideoID, video.ChannelName, video.Title, len(tr.Cues))
	return 0
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

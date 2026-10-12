package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/PatAkil/youtube-transcripts/internal/corpus"
	"github.com/PatAkil/youtube-transcripts/internal/youtube"
)

// Fetch stores one Video's Transcript and metadata in the Corpus. It returns
// the process exit code.
func Fetch(ctx context.Context, deps Deps, corpusDir string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "fetch: expected exactly one video ID or URL")
		return 1
	}
	videoID, err := youtube.ParseVideoID(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "fetch: %v\n", err)
		return 1
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
	videoJSON, err := corpus.MarshalJSON(video)
	if err != nil {
		fmt.Fprintf(stderr, "fetch %s: %v\n", videoID, err)
		return 1
	}
	files := map[string][]byte{
		corpus.VideoFile:      videoJSON,
		corpus.TranscriptFile: []byte(renderTranscript(tr.Cues)),
	}
	if err := corpus.WriteVideoDir(corpusDir, video.ChannelID, video.VideoID, files, nil); err != nil {
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

package youtube

import (
	"context"
	"fmt"
)

// Fake is an in-memory Client keyed by video and channel ID. Tests fill the
// maps with canned Transcripts, feeds or errors; an unknown ID is an error.
type Fake struct {
	Transcripts map[string]Transcript
	Feeds       map[string][]VideoMeta
	// Errors makes FetchTranscript (keyed by video ID) or ListNewestVideos
	// (keyed by channel ID) fail with the given error.
	Errors map[string]error
}

var _ Client = (*Fake)(nil)

// ListNewestVideos returns the canned feed for channelID.
func (f *Fake) ListNewestVideos(_ context.Context, channelID string) ([]VideoMeta, error) {
	if err, ok := f.Errors[channelID]; ok {
		return nil, err
	}
	feed, ok := f.Feeds[channelID]
	if !ok {
		return nil, fmt.Errorf("fake youtube: unknown channel %q", channelID)
	}
	return feed, nil
}

// FetchTranscript returns the canned Transcript for videoID.
func (f *Fake) FetchTranscript(_ context.Context, videoID string) (Transcript, error) {
	if err, ok := f.Errors[videoID]; ok {
		return Transcript{}, err
	}
	tr, ok := f.Transcripts[videoID]
	if !ok {
		return Transcript{}, fmt.Errorf("fake youtube: unknown video %q", videoID)
	}
	return tr, nil
}

package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/PatAkil/youtube-transcripts/internal/corpus"
)

const syncUsage = `Fetch new Videos from every Followed Channel.

Usage:
  youtube-transcripts [--corpus DIR] sync

The Followed Channels are read from <corpus>/followed-channels.json, a
hand-edited list of {"channel_id": "UC...", "name": "Display Name"} entries.
For each Followed Channel the newest Videos in its public feed are listed and
every Video not already in the Corpus is fetched exactly as "fetch" would.
Videos already present are skipped, so running sync repeatedly is safe.

A Channel whose feed can't be read, or a Video whose fetch fails, is named in
the closing summary and the run continues. Exit status is 0 when at least one
Channel was walked, 1 when every Channel failed or there are no Followed
Channels to walk.

Flags:
  --help     Show this help
`

// Sync walks every Followed Channel and fetches the Videos not yet in the
// Corpus. It returns the process exit code.
func Sync(ctx context.Context, deps Deps, corpusDir string, args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprint(stdout, syncUsage)
			return 0
		default:
			fmt.Fprintf(stderr, "sync: unexpected argument %q\n\n%s", a, syncUsage)
			return 1
		}
	}

	channels, err := corpus.ReadFollowedChannels(corpusDir)
	if err != nil {
		fmt.Fprintf(stderr, "sync: %v\n", err)
		return 1
	}

	var walked, fetched, skipped, failed int
	var failedChannels, failedVideos []string
	for _, ch := range channels {
		videos, err := deps.YouTube.ListNewestVideos(ctx, ch.ChannelID)
		if err != nil {
			fmt.Fprintf(stderr, "sync: channel %s (%s): %v\n", ch.Name, ch.ChannelID, err)
			failedChannels = append(failedChannels, fmt.Sprintf("%s (%s)", ch.Name, ch.ChannelID))
			continue
		}
		walked++
		for _, v := range videos {
			res, err := fetchVideo(ctx, deps, corpusDir, v.VideoID, false)
			if err != nil {
				fmt.Fprintf(stderr, "sync: video %s (%s): %v\n", v.VideoID, ch.Name, err)
				failedVideos = append(failedVideos, fmt.Sprintf("%s (%s)", v.VideoID, ch.Name))
				failed++
				continue
			}
			if res.AlreadyPresent {
				skipped++
				continue
			}
			fetched++
			fmt.Fprintln(stdout, res.Summary())
		}
	}

	fmt.Fprintf(stdout, "\nchannels walked: %d\nvideos fetched: %d\nvideos skipped (already present): %d\nvideos failed: %d\n",
		walked, fetched, skipped, failed)
	if len(failedChannels) > 0 {
		fmt.Fprintf(stdout, "failed channels: %s\n", strings.Join(failedChannels, ", "))
	}
	if len(failedVideos) > 0 {
		fmt.Fprintf(stdout, "failed videos: %s\n", strings.Join(failedVideos, ", "))
	}
	return 0
}

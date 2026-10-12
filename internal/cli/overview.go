package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PatAkil/youtube-transcripts/internal/corpus"
)

const overviewUsage = `Write the Sponsorship overview from the Segments files in the Corpus.

Usage:
  youtube-transcripts [--corpus DIR] overview

Walks every Channel directory and every Video directory beneath it, reads
video.json and segments.json, and writes <corpus>/overview.md with four
sections: By Sponsor (Sponsor -> Channel -> Videos with Segment start times),
By Channel (Channel -> Sponsor -> Segment count), Awaiting agent pass (Videos
with no segments.json yet) and No transcript (Videos with transcript "none";
these are not counted as awaiting, since there is nothing for the agent to
read). Each Segment start is a [mm:ss] link into the Video at that second.

A video.json or segments.json that fails to parse is named on stderr and
skipped; the overview is still written and the exit code stays 0. The output
is stable, so running overview twice on an unchanged Corpus changes nothing.

Flags:
  --help     Show this help
`

// OverviewFile is the Markdown file overview writes at the Corpus root.
const OverviewFile = "overview.md"

// Overview writes the Sponsorship overview for the Corpus. It returns the
// process exit code.
func Overview(ctx context.Context, deps Deps, corpusDir string, args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprint(stdout, overviewUsage)
			return 0
		default:
			fmt.Fprintf(stderr, "overview: unexpected argument %q\n\n%s", a, overviewUsage)
			return 1
		}
	}

	videos, err := scanCorpus(corpusDir, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "overview: %v\n", err)
		return 1
	}
	ov := buildOverview(videos)
	path := filepath.Join(corpusDir, OverviewFile)
	if err := writeFileAtomic(path, []byte(ov.render())); err != nil {
		fmt.Fprintf(stderr, "overview: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s  sponsors=%d  channels=%d  awaiting=%d\n", path, len(ov.sponsors), len(ov.channels), ov.awaitingCount())
	return 0
}

// scannedVideo is one Video directory as the walk found it. Segments is nil
// when the Video has no segments.json (awaiting an agent pass).
type scannedVideo struct {
	Meta     corpus.Video
	Segments *corpus.Segments
}

// scanCorpus walks <corpus>/<channel>/<video>/ and reads each Video's files.
// A file that fails to parse is reported on stderr and its Video skipped.
func scanCorpus(corpusDir string, stderr io.Writer) ([]scannedVideo, error) {
	channels, err := os.ReadDir(corpusDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan corpus: %w", err)
	}
	var videos []scannedVideo
	for _, ch := range channels {
		if !ch.IsDir() || strings.HasPrefix(ch.Name(), ".") {
			continue
		}
		channelDir := filepath.Join(corpusDir, ch.Name())
		entries, err := os.ReadDir(channelDir)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", channelDir, err)
		}
		for _, v := range entries {
			if !v.IsDir() || strings.HasPrefix(v.Name(), ".") {
				continue
			}
			videoDir := filepath.Join(channelDir, v.Name())
			sv, err := readVideoDir(videoDir)
			if err != nil {
				fmt.Fprintf(stderr, "overview: %v; Video skipped\n", err)
				continue
			}
			videos = append(videos, sv)
		}
	}
	return videos, nil
}

func readVideoDir(videoDir string) (scannedVideo, error) {
	metaPath := filepath.Join(videoDir, corpus.VideoFile)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return scannedVideo{}, err
	}
	var meta corpus.Video
	if err := json.Unmarshal(data, &meta); err != nil {
		return scannedVideo{}, fmt.Errorf("parse %s: %w", metaPath, err)
	}
	if meta.VideoID == "" {
		return scannedVideo{}, fmt.Errorf("parse %s: missing video_id", metaPath)
	}
	sv := scannedVideo{Meta: meta}

	segPath := filepath.Join(videoDir, corpus.SegmentsFile)
	if _, err := os.Stat(segPath); err == nil {
		segs, err := corpus.ReadSegments(segPath)
		if err != nil {
			return scannedVideo{}, err
		}
		sv.Segments = &segs
	} else if !os.IsNotExist(err) {
		return scannedVideo{}, err
	}
	return sv, nil
}

// overview is the grouped, sorted content of overview.md.
type overview struct {
	sponsors     []*sponsorEntry
	channels     []*channelEntry  // Channels with at least one Segment
	awaiting     []*channelVideos // Videos with no segments.json, by Channel
	noTranscript []videoRef       // Videos with transcript "none"
}

type sponsorEntry struct {
	Name     string
	Channels []*channelVideos
}

type channelVideos struct {
	ID     string
	Name   string
	Videos []videoRef
}

type videoRef struct {
	Meta   corpus.Video
	Starts []float64 // Segment start times for this Sponsor in this Video
}

type channelEntry struct {
	ID       string
	Name     string
	Sponsors []sponsorCount
}

type sponsorCount struct {
	Name  string
	Count int
}

func buildOverview(videos []scannedVideo) overview {
	// Group Segments: sponsor key -> channel ID -> video ID -> starts.
	type videoStarts struct {
		meta   corpus.Video
		starts []float64
	}
	bySponsor := map[string]map[string]map[string]*videoStarts{}
	sponsorSpellings := map[string]map[string]int{} // key -> spelling -> uses
	channelNames := map[string]string{}
	channelSponsorCount := map[string]map[string]int{}
	awaitingByChannel := map[string]*channelVideos{}
	var noTranscript []videoRef

	for _, v := range videos {
		chID := v.Meta.ChannelID
		channelNames[chID] = v.Meta.ChannelName
		if v.Meta.Transcript == corpus.TranscriptNone {
			noTranscript = append(noTranscript, videoRef{Meta: v.Meta})
			if v.Segments == nil {
				// Nothing for the agent pass to read; the No transcript
				// section is where this Video is waiting.
				continue
			}
		}
		if v.Segments == nil {
			cv := awaitingByChannel[chID]
			if cv == nil {
				cv = &channelVideos{ID: chID}
				awaitingByChannel[chID] = cv
			}
			cv.Videos = append(cv.Videos, videoRef{Meta: v.Meta})
			continue
		}
		for _, s := range v.Segments.Segments {
			name := strings.TrimSpace(s.Sponsor)
			key := strings.ToLower(name)
			if sponsorSpellings[key] == nil {
				sponsorSpellings[key] = map[string]int{}
			}
			sponsorSpellings[key][name]++
			if bySponsor[key] == nil {
				bySponsor[key] = map[string]map[string]*videoStarts{}
			}
			if bySponsor[key][chID] == nil {
				bySponsor[key][chID] = map[string]*videoStarts{}
			}
			vs := bySponsor[key][chID][v.Meta.VideoID]
			if vs == nil {
				vs = &videoStarts{meta: v.Meta}
				bySponsor[key][chID][v.Meta.VideoID] = vs
			}
			vs.starts = append(vs.starts, s.Start)
			if channelSponsorCount[chID] == nil {
				channelSponsorCount[chID] = map[string]int{}
			}
			channelSponsorCount[chID][key]++
		}
	}

	var ov overview
	sponsorNames := map[string]string{}
	for key, spellings := range sponsorSpellings {
		sponsorNames[key] = mostFrequent(spellings)
	}

	for key, channels := range bySponsor {
		se := &sponsorEntry{Name: sponsorNames[key]}
		for chID, vids := range channels {
			cv := &channelVideos{ID: chID, Name: channelNames[chID]}
			for _, vs := range vids {
				sort.Float64s(vs.starts)
				cv.Videos = append(cv.Videos, videoRef{Meta: vs.meta, Starts: vs.starts})
			}
			sortVideoRefs(cv.Videos)
			se.Channels = append(se.Channels, cv)
		}
		sortChannelVideos(se.Channels)
		ov.sponsors = append(ov.sponsors, se)
	}
	sort.Slice(ov.sponsors, func(i, j int) bool {
		return lessName(ov.sponsors[i].Name, ov.sponsors[j].Name)
	})

	for chID, counts := range channelSponsorCount {
		ce := &channelEntry{ID: chID, Name: channelNames[chID]}
		for key, n := range counts {
			ce.Sponsors = append(ce.Sponsors, sponsorCount{Name: sponsorNames[key], Count: n})
		}
		sort.Slice(ce.Sponsors, func(i, j int) bool {
			return lessName(ce.Sponsors[i].Name, ce.Sponsors[j].Name)
		})
		ov.channels = append(ov.channels, ce)
	}
	sort.Slice(ov.channels, func(i, j int) bool {
		a, b := ov.channels[i], ov.channels[j]
		if a.Name != b.Name {
			return lessName(a.Name, b.Name)
		}
		return a.ID < b.ID
	})

	for chID, cv := range awaitingByChannel {
		cv.Name = channelNames[chID]
		sortVideoRefs(cv.Videos)
		ov.awaiting = append(ov.awaiting, cv)
	}
	sortChannelVideos(ov.awaiting)

	sortVideoRefs(noTranscript)
	ov.noTranscript = noTranscript
	return ov
}

func (ov overview) awaitingCount() int {
	n := 0
	for _, cv := range ov.awaiting {
		n += len(cv.Videos)
	}
	return n
}

// mostFrequent picks the spelling used most often; a tie goes to the
// alphabetically first spelling so the choice is stable.
func mostFrequent(uses map[string]int) string {
	best, bestN := "", -1
	for spelling, n := range uses {
		if n > bestN || (n == bestN && spelling < best) {
			best, bestN = spelling, n
		}
	}
	return best
}

// lessName orders display names case-insensitively, falling back to the
// exact spelling so the order is total.
func lessName(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}

func sortChannelVideos(cs []*channelVideos) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Name != cs[j].Name {
			return lessName(cs[i].Name, cs[j].Name)
		}
		return cs[i].ID < cs[j].ID
	})
}

// sortVideoRefs orders Videos by publish date, then by ID.
func sortVideoRefs(vs []videoRef) {
	sort.Slice(vs, func(i, j int) bool {
		a, b := vs[i].Meta, vs[j].Meta
		if !a.Published.Equal(b.Published) {
			return a.Published.Before(b.Published)
		}
		return a.VideoID < b.VideoID
	})
}

func (ov overview) render() string {
	var b strings.Builder
	b.WriteString("# Sponsorship overview\n")

	b.WriteString("\n## By Sponsor\n")
	if len(ov.sponsors) == 0 {
		b.WriteString("\n_None._\n")
	}
	for _, se := range ov.sponsors {
		fmt.Fprintf(&b, "\n### %s\n\n", se.Name)
		for _, cv := range se.Channels {
			fmt.Fprintf(&b, "- %s\n", channelLabel(cv.ID, cv.Name))
			for _, v := range cv.Videos {
				fmt.Fprintf(&b, "  - %s: %s\n", videoLabel(v.Meta), startLinks(v.Meta.VideoID, v.Starts))
			}
		}
	}

	b.WriteString("\n## By Channel\n")
	if len(ov.channels) == 0 {
		b.WriteString("\n_None._\n")
	}
	for _, ce := range ov.channels {
		fmt.Fprintf(&b, "\n### %s\n\n", channelLabel(ce.ID, ce.Name))
		for _, sc := range ce.Sponsors {
			fmt.Fprintf(&b, "- %s: %s\n", sc.Name, plural(sc.Count, "Segment"))
		}
	}

	b.WriteString("\n## Awaiting agent pass\n")
	if len(ov.awaiting) == 0 {
		b.WriteString("\n_None._\n")
	}
	for _, cv := range ov.awaiting {
		fmt.Fprintf(&b, "\n### %s\n\n", channelLabel(cv.ID, cv.Name))
		for _, v := range cv.Videos {
			fmt.Fprintf(&b, "- %s\n", videoLabel(v.Meta))
		}
	}

	b.WriteString("\n## No transcript\n\n")
	if len(ov.noTranscript) == 0 {
		b.WriteString("_None._\n")
	}
	for _, v := range ov.noTranscript {
		fmt.Fprintf(&b, "- %s: %s\n", channelLabel(v.Meta.ChannelID, v.Meta.ChannelName), videoLabel(v.Meta))
	}
	return b.String()
}

func channelLabel(id, name string) string {
	if name == "" {
		return id
	}
	return fmt.Sprintf("%s (%s)", name, id)
}

func videoLabel(v corpus.Video) string {
	if v.Title == "" {
		return v.VideoID
	}
	return fmt.Sprintf("%s (%s)", v.Title, v.VideoID)
}

// startLinks renders each Segment start as a [mm:ss] timestamp linking to
// the Video at that second, so a read can be opened directly.
func startLinks(videoID string, starts []float64) string {
	parts := make([]string, 0, len(starts))
	for _, s := range starts {
		secs := int(s)
		d := time.Duration(secs) * time.Second
		parts = append(parts, fmt.Sprintf("[%s](https://www.youtube.com/watch?v=%s&t=%ds)", corpus.Timestamp(d), videoID, secs))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// writeFileAtomic writes data to a temporary sibling and renames it over path.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

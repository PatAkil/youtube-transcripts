package youtube

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HTTPClient is the key-free YouTube implementation. It reads the public
// watch page the web player loads and the track URL that page points at.
type HTTPClient struct {
	http *http.Client
}

var _ Client = (*HTTPClient)(nil)

// NewClient wraps httpClient (nil means http.DefaultClient).
func NewClient(httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &HTTPClient{http: httpClient}
}

const (
	watchURL  = "https://www.youtube.com/watch"
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

// ListNewestVideos is filled in by the sync ticket.
func (c *HTTPClient) ListNewestVideos(_ context.Context, channelID string) ([]VideoMeta, error) {
	return nil, fmt.Errorf("youtube: listing videos of channel %s is not available yet", channelID)
}

// FetchTranscript loads the watch page, picks the English track (manual
// preferred over auto-generated) and downloads it as cues.
func (c *HTTPClient) FetchTranscript(ctx context.Context, videoID string) (Transcript, error) {
	page, err := c.get(ctx, watchURL+"?v="+videoID+"&hl=en")
	if err != nil {
		return Transcript{}, err
	}
	pr, err := extractPlayerResponse(page)
	if err != nil {
		return Transcript{}, fmt.Errorf("youtube: %s: %w", videoID, err)
	}
	meta, err := pr.meta(videoID)
	if err != nil {
		return Transcript{}, fmt.Errorf("youtube: %s: %w", videoID, err)
	}

	track, kind := pr.englishTrack()
	tr := Transcript{Meta: meta, Track: kind}
	if kind == TrackNone {
		return tr, nil
	}
	body, err := c.get(ctx, track.BaseURL)
	if err != nil {
		return Transcript{}, fmt.Errorf("youtube: %s: track download: %w", videoID, err)
	}
	cues, err := parseTimedText(body)
	if err != nil {
		return Transcript{}, fmt.Errorf("youtube: %s: track: %w", videoID, err)
	}
	tr.Cues = cues
	return tr, nil
}

func (c *HTTPClient) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cookie", "CONSENT=YES+1")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube: GET %s: HTTP %d", redact(rawURL), resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

// redact drops the query string from a URL for error messages.
func redact(rawURL string) string {
	if i := strings.IndexByte(rawURL, '?'); i >= 0 {
		return rawURL[:i]
	}
	return rawURL
}

// playerResponse is the slice of ytInitialPlayerResponse the CLI reads.
type playerResponse struct {
	VideoDetails struct {
		VideoID   string `json:"videoId"`
		Title     string `json:"title"`
		ChannelID string `json:"channelId"`
		Author    string `json:"author"`
	} `json:"videoDetails"`
	Microformat struct {
		Renderer struct {
			PublishDate string `json:"publishDate"`
			UploadDate  string `json:"uploadDate"`
		} `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
	TrackList struct {
		Renderer struct {
			Tracks []trackEntry `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
}

// trackEntry is one entry of YouTube's track list (its "captionTracks"). Kind is "asr" for an
// auto-generated track and absent for one the Channel uploaded.
type trackEntry struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Kind         string `json:"kind"`
}

var playerResponseMarker = regexp.MustCompile(`ytInitialPlayerResponse\s*=\s*\{`)

var errNoPlayerResponse = errors.New("watch page has no player response (consent interstitial, bot check or layout change)")

// extractPlayerResponse finds `ytInitialPlayerResponse = {...}` in the page
// and decodes it. The object is located by brace matching that honours JSON
// string literals, since titles and descriptions may contain braces.
func extractPlayerResponse(page []byte) (*playerResponse, error) {
	loc := playerResponseMarker.FindIndex(page)
	if loc == nil {
		return nil, errNoPlayerResponse
	}
	start := loc[1] - 1
	end, ok := matchBraces(page, start)
	if !ok {
		return nil, errNoPlayerResponse
	}
	var pr playerResponse
	if err := json.Unmarshal(page[start:end], &pr); err != nil {
		return nil, fmt.Errorf("decode player response: %w", err)
	}
	return &pr, nil
}

// matchBraces returns the index just past the brace closing the one at start.
func matchBraces(b []byte, start int) (int, bool) {
	depth := 0
	inString := false
	for i := start; i < len(b); i++ {
		c := b[i]
		if inString {
			switch c {
			case '\\':
				i++
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func (pr *playerResponse) meta(videoID string) (VideoMeta, error) {
	d := pr.VideoDetails
	if d.VideoID == "" || d.ChannelID == "" {
		return VideoMeta{}, errors.New("player response carries no video details (video unavailable or private?)")
	}
	m := VideoMeta{VideoID: d.VideoID, ChannelID: d.ChannelID, ChannelName: d.Author, Title: d.Title}
	if m.VideoID != videoID {
		return VideoMeta{}, fmt.Errorf("watch page is for video %s", m.VideoID)
	}
	date := pr.Microformat.Renderer.PublishDate
	if date == "" {
		date = pr.Microformat.Renderer.UploadDate
	}
	if date != "" {
		m.Published = parseDate(date)
	}
	return m, nil
}

// parseDate accepts the YYYY-MM-DD and RFC 3339 forms YouTube uses. An
// unparseable date is left zero rather than failing the fetch.
func parseDate(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// englishTrack picks the first manual en / en-* track, else the first
// auto-generated one, else reports TrackNone.
func (pr *playerResponse) englishTrack() (trackEntry, TrackKind) {
	tracks := pr.TrackList.Renderer.Tracks
	for _, t := range tracks {
		if isEnglish(t.LanguageCode) && t.Kind != "asr" {
			return t, TrackManual
		}
	}
	for _, t := range tracks {
		if isEnglish(t.LanguageCode) && t.Kind == "asr" {
			return t, TrackAuto
		}
	}
	return trackEntry{}, TrackNone
}

func isEnglish(code string) bool {
	code = strings.ToLower(code)
	return code == "en" || strings.HasPrefix(code, "en-")
}

// timedText covers both shapes the track URL answers with: the classic
// <transcript><text start= dur=> and the srv3 <timedtext><body><p t= d=>.
type timedText struct {
	Texts []struct {
		Start string `xml:"start,attr"`
		Body  string `xml:",innerxml"`
	} `xml:"text"`
	Paragraphs []struct {
		T    string `xml:"t,attr"`
		Body string `xml:",innerxml"`
	} `xml:"body>p"`
}

var xmlTag = regexp.MustCompile(`<[^>]*>`)

// parseTimedText decodes a track into cues, dropping empty ones. Text is
// unescaped twice because YouTube often double-escapes entities.
func parseTimedText(body []byte) ([]Cue, error) {
	var tt timedText
	if err := xml.Unmarshal(body, &tt); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	var cues []Cue
	add := func(start time.Duration, raw string) {
		text := cleanCueText(raw)
		if text != "" {
			cues = append(cues, Cue{Start: start, Text: text})
		}
	}
	for _, t := range tt.Texts {
		secs, err := strconv.ParseFloat(t.Start, 64)
		if err != nil {
			return nil, fmt.Errorf("bad cue start %q", t.Start)
		}
		add(time.Duration(secs*float64(time.Second)+0.5), t.Body)
	}
	for _, p := range tt.Paragraphs {
		ms, err := strconv.ParseInt(p.T, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad cue start %q", p.T)
		}
		add(time.Duration(ms)*time.Millisecond, p.Body)
	}
	return cues, nil
}

func cleanCueText(raw string) string {
	s := xmlTag.ReplaceAllString(raw, " ")
	s = html.UnescapeString(html.UnescapeString(s))
	return strings.Join(strings.Fields(s), " ")
}

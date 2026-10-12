package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HTTPClient is the key-free YouTube implementation. It reads the public
// watch page the web player loads and the track URL that page points at,
// falling back to the Android app's player API when that URL answers empty.
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
	feedURL   = "https://www.youtube.com/feeds/videos.xml"
	playerURL = "https://www.youtube.com/youtubei/v1/player"
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"

	// The Android app's player API client. Track URLs handed to the web
	// player now need a proof-of-origin token and answer an empty body
	// without one; the Android client's track URLs do not.
	androidClientName    = "ANDROID"
	androidClientVersion = "20.10.38"
	androidUserAgent     = "com.google.android.youtube/" + androidClientVersion + " (Linux; U; Android 11) gzip"
)

// ListNewestVideos reads the Channel's public Atom feed, which lists its
// newest ~15 Videos. A mistyped channel ID answers 404 and is an error.
func (c *HTTPClient) ListNewestVideos(ctx context.Context, channelID string) ([]VideoMeta, error) {
	q := url.Values{"channel_id": {channelID}}
	body, err := c.get(ctx, feedURL+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("youtube: channel %s feed: %w", channelID, err)
	}
	videos, err := parseChannelFeed(body, channelID)
	if err != nil {
		return nil, fmt.Errorf("youtube: channel %s feed: %w", channelID, err)
	}
	return videos, nil
}

// channelFeed is the slice of the Atom feed the CLI reads. Entry fields in
// the yt: namespace are matched by local name, which encoding/xml allows.
type channelFeed struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Entries []struct {
		VideoID   string `xml:"videoId"`
		ChannelID string `xml:"channelId"`
		Title     string `xml:"title"`
		Author    string `xml:"author>name"`
		Published string `xml:"published"`
	} `xml:"entry"`
}

// parseChannelFeed decodes a channel feed into Video metadata in feed order
// (newest first). An entry without a video ID is skipped.
func parseChannelFeed(body []byte, channelID string) ([]VideoMeta, error) {
	var feed channelFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	videos := make([]VideoMeta, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		if e.VideoID == "" {
			continue
		}
		m := VideoMeta{
			VideoID:     e.VideoID,
			ChannelID:   e.ChannelID,
			ChannelName: e.Author,
			Title:       e.Title,
			Published:   parseDate(e.Published),
		}
		if m.ChannelID == "" {
			m.ChannelID = channelID
		}
		if m.ChannelName == "" {
			m.ChannelName = feed.Title
		}
		videos = append(videos, m)
	}
	return videos, nil
}

// FetchTranscript loads the watch page, picks the English track (manual
// preferred over auto-generated) and downloads it as cues.
func (c *HTTPClient) FetchTranscript(ctx context.Context, videoID string) (Transcript, error) {
	// videoID may come straight from a feed, so it is escaped rather than
	// interpolated.
	q := url.Values{"v": {videoID}, "hl": {"en"}}
	page, err := c.get(ctx, watchURL+"?"+q.Encode())
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
	if len(bytes.TrimSpace(body)) == 0 {
		// The web player's track URL needs a proof-of-origin token we do
		// not have; the Android client's track URL for the same Video
		// does not.
		body, err = c.androidTrack(ctx, videoID)
		if err != nil {
			return Transcript{}, fmt.Errorf("youtube: %s: %w", videoID, err)
		}
	}
	cues, err := parseTimedText(body)
	if err != nil {
		return Transcript{}, fmt.Errorf("youtube: %s: track: %w", videoID, err)
	}
	tr.Cues = cues
	return tr, nil
}

// androidTrack asks the player API as the Android app for the Video and
// downloads its English track. It is the fallback for a web track URL that
// answers an empty body.
func (c *HTTPClient) androidTrack(ctx context.Context, videoID string) ([]byte, error) {
	reqBody, err := json.Marshal(map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":        androidClientName,
				"clientVersion":     androidClientVersion,
				"androidSdkVersion": 30,
				"userAgent":         androidUserAgent,
				"hl":                "en",
				"gl":                "US",
			},
		},
		"videoId":        videoID,
		"contentCheckOk": true,
		"racyCheckOk":    true,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, playerURL+"?prettyPrint=false", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", androidUserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-YouTube-Client-Name", "3")
	req.Header.Set("X-YouTube-Client-Version", androidClientVersion)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("player API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("player API: HTTP %d", resp.StatusCode)
	}
	var pr playerResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&pr); err != nil {
		return nil, fmt.Errorf("player API: decode: %w", err)
	}
	track, kind := pr.englishTrack()
	if kind == TrackNone {
		return nil, errors.New("player API lists no English track although the watch page does")
	}
	body, err := c.get(ctx, track.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("track download: %w", err)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("track URL answered an empty body for both the web and the Android client (YouTube may have changed its track route)")
	}
	return body, nil
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

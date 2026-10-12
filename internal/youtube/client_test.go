package youtube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rewriteTransport sends every request to the test server regardless of the
// host the client asked for, so fixtures can carry real youtube.com URLs.
type rewriteTransport struct{ target *url.URL }

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.URL.Scheme = rt.target.Scheme
	r.URL.Host = rt.target.Host
	r.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(r)
}

type fakeYouTube struct {
	watchPages map[string]string // video ID -> fixture file for /watch
	timedText  map[string]string // lang -> fixture file for /api/timedtext
	watchCode  int
}

func newTestClient(t *testing.T, fy fakeYouTube) *HTTPClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			if fy.watchCode != 0 {
				w.WriteHeader(fy.watchCode)
				return
			}
			fixture, ok := fy.watchPages[r.URL.Query().Get("v")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			serveFixture(t, w, fixture)
		case "/api/timedtext":
			fixture, ok := fy.timedText[r.URL.Query().Get("lang")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			serveFixture(t, w, fixture)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	return NewClient(&http.Client{Transport: rewriteTransport{target: target}})
}

func serveFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Errorf("fixture %s: %v", name, err)
		http.Error(w, err.Error(), 500)
		return
	}
	_, _ = w.Write(b)
}

func TestHTTPClientPrefersManualEnglishTrack(t *testing.T) {
	c := newTestClient(t, fakeYouTube{
		watchPages: map[string]string{"dQw4w9WgXcQ": "watch_tracks.html"},
		timedText:  map[string]string{"en-GB": "timedtext_en-GB.xml", "en": "timedtext_srv3.xml"},
	})

	tr, err := c.FetchTranscript(context.Background(), "dQw4w9WgXcQ")
	if err != nil {
		t.Fatal(err)
	}
	wantMeta := VideoMeta{
		VideoID:     "dQw4w9WgXcQ",
		ChannelID:   "UCabc123",
		ChannelName: "Example Channel",
		Title:       `An example video {with braces} and "quotes"`,
		Published:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	if tr.Meta != wantMeta {
		t.Errorf("Meta = %+v, want %+v", tr.Meta, wantMeta)
	}
	if tr.Track != TrackManual {
		t.Errorf("Track = %q, want manual", tr.Track)
	}
	wantCues := []Cue{
		{Start: 0, Text: "hello and welcome"},
		{Start: 12340 * time.Millisecond, Text: "this video is sponsored by Acme & Co 'today'"},
		{Start: 3660250 * time.Millisecond, Text: "see you next time"},
	}
	assertCues(t, tr.Cues, wantCues)
}

func TestHTTPClientFallsBackToAutoTrackAndReadsSrv3(t *testing.T) {
	c := newTestClient(t, fakeYouTube{
		watchPages: map[string]string{"autoOnly123": "watch_auto_only.html"},
		timedText:  map[string]string{"en": "timedtext_srv3.xml"},
	})

	tr, err := c.FetchTranscript(context.Background(), "autoOnly123")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Track != TrackAuto {
		t.Errorf("Track = %q, want auto", tr.Track)
	}
	if want := time.Date(2023, 5, 6, 7, 8, 9, 0, time.FixedZone("", -7*3600)); !tr.Meta.Published.Equal(want) {
		t.Errorf("Published = %v, want %v", tr.Meta.Published, want)
	}
	assertCues(t, tr.Cues, []Cue{
		{Start: 0, Text: "hello and welcome"},
		{Start: 12340 * time.Millisecond, Text: "this video is sponsored by Acme & Co"},
	})
}

func TestHTTPClientReportsNoEnglishTrackWithMetadata(t *testing.T) {
	c := newTestClient(t, fakeYouTube{watchPages: map[string]string{"noTrack1234": "watch_no_tracks.html"}})

	tr, err := c.FetchTranscript(context.Background(), "noTrack1234")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Track != TrackNone || len(tr.Cues) != 0 {
		t.Errorf("Track = %q with %d cues, want none and no cues", tr.Track, len(tr.Cues))
	}
	if tr.Meta.ChannelID != "UCsilent" || tr.Meta.Title != "Silent film" || tr.Meta.VideoID != "noTrack1234" {
		t.Errorf("Meta = %+v", tr.Meta)
	}
}

func TestHTTPClientErrorsWhenPlayerResponseIsMissing(t *testing.T) {
	c := newTestClient(t, fakeYouTube{watchPages: map[string]string{"consent1234": "watch_consent.html"}})
	_, err := c.FetchTranscript(context.Background(), "consent1234")
	if err == nil || !strings.Contains(err.Error(), "player response") {
		t.Errorf("err = %v, want one mentioning the missing player response", err)
	}
}

func TestHTTPClientErrorsOnNonOKWatchPage(t *testing.T) {
	c := newTestClient(t, fakeYouTube{watchCode: http.StatusTooManyRequests})
	_, err := c.FetchTranscript(context.Background(), "dQw4w9WgXcQ")
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Errorf("err = %v, want one carrying the HTTP status", err)
	}
}

func TestHTTPClientErrorsWhenTrackDownloadFails(t *testing.T) {
	c := newTestClient(t, fakeYouTube{watchPages: map[string]string{"dQw4w9WgXcQ": "watch_tracks.html"}})
	_, err := c.FetchTranscript(context.Background(), "dQw4w9WgXcQ")
	if err == nil {
		t.Error("want an error when the track URL answers 404")
	}
}

func assertCues(t *testing.T, got, want []Cue) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d cues %+v, want %d %+v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cue %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestLiveFetchTranscript reaches real YouTube. Run with YT_LIVE=1.
func TestLiveFetchTranscript(t *testing.T) {
	if os.Getenv("YT_LIVE") != "1" {
		t.Skip("set YT_LIVE=1 to run the live YouTube smoke test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := NewClient(&http.Client{Timeout: 30 * time.Second})

	// "Me at the zoo", the first video on YouTube: stable, short, has tracks.
	tr, err := c.FetchTranscript(ctx, "jNQXAC9IVRw")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Meta.VideoID != "jNQXAC9IVRw" || tr.Meta.ChannelID == "" || tr.Meta.Title == "" {
		t.Errorf("Meta = %+v", tr.Meta)
	}
	if tr.Track != TrackNone && len(tr.Cues) == 0 {
		t.Errorf("track %q reported but no cues", tr.Track)
	}
}

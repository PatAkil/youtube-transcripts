package sponsorblock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// newTestClient serves body with status for every skip-segments request whose
// query asks for the sponsor category of videoID; anything else is a 400, so a
// wrong request shape shows up as a test failure rather than canned data.
func newTestClient(t *testing.T, videoID string, status int, body string) *HTTPClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/skipSegments" || q.Get("videoID") != videoID || q.Get("category") != "sponsor" {
			http.Error(w, "unexpected request "+r.URL.String(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.Client())
	c.baseURL = srv.URL
	return c
}

func TestFetchSlotsReturnsSponsorWindows(t *testing.T) {
	body := `[
	  {"segment":[12.345,87.0],"UUID":"uuid-a","category":"sponsor","actionType":"skip","videoDuration":600,"locked":0,"votes":3,"description":""},
	  {"segment":[300,330.5],"UUID":"uuid-b","category":"sponsor","actionType":"skip","videoDuration":600,"locked":1,"votes":10,"description":""}
	]`
	c := newTestClient(t, "dQw4w9WgXcQ", http.StatusOK, body)

	got, err := c.FetchSlots(context.Background(), "dQw4w9WgXcQ")

	if err != nil {
		t.Fatal(err)
	}
	want := []Window{{Start: 12.345, End: 87, ID: "uuid-a"}, {Start: 300, End: 330.5, ID: "uuid-b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FetchSlots = %+v, want %+v", got, want)
	}
}

func TestFetchSlotsTreatsNotFoundAsNoData(t *testing.T) {
	c := newTestClient(t, "dQw4w9WgXcQ", http.StatusNotFound, "Not Found")

	got, err := c.FetchSlots(context.Background(), "dQw4w9WgXcQ")

	if err != nil {
		t.Fatalf("404 must not be an error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("FetchSlots = %+v, want no windows", got)
	}
}

func TestFetchSlotsFailsOnOtherStatus(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusTooManyRequests, http.StatusBadRequest} {
		c := newTestClient(t, "dQw4w9WgXcQ", status, "[]")
		got, err := c.FetchSlots(context.Background(), "dQw4w9WgXcQ")
		if err == nil {
			t.Errorf("HTTP %d: want an error, got windows %+v", status, got)
			continue
		}
		if !strings.Contains(err.Error(), strconv.Itoa(status)) {
			t.Errorf("HTTP %d: error %q does not name the status", status, err)
		}
	}
}

func TestFetchSlotsKeepsOnlySponsorCategory(t *testing.T) {
	body := `[
	  {"segment":[0,5.5],"UUID":"intro","category":"intro"},
	  {"segment":[12.5,87],"UUID":"uuid-a","category":"sponsor"},
	  {"segment":[500,520],"UUID":"self","category":"selfpromo"},
	  {"segment":[590,600],"UUID":"interaction","category":"interaction"}
	]`
	c := newTestClient(t, "dQw4w9WgXcQ", http.StatusOK, body)

	got, err := c.FetchSlots(context.Background(), "dQw4w9WgXcQ")

	if err != nil {
		t.Fatal(err)
	}
	want := []Window{{Start: 12.5, End: 87, ID: "uuid-a"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FetchSlots = %+v, want %+v", got, want)
	}
}

func TestFetchSlotsRejectsMalformedSegment(t *testing.T) {
	for name, body := range map[string]string{
		"one bound":   `[{"segment":[12.5],"UUID":"uuid-a","category":"sponsor"}]`,
		"not a list":  `{"segment":[12.5,87],"UUID":"uuid-a","category":"sponsor"}`,
		"end < start": `[{"segment":[87,12.5],"UUID":"uuid-a","category":"sponsor"}]`,
	} {
		c := newTestClient(t, "dQw4w9WgXcQ", http.StatusOK, body)
		got, err := c.FetchSlots(context.Background(), "dQw4w9WgXcQ")
		if err == nil {
			t.Errorf("%s: want an error, got windows %+v", name, got)
		}
	}
}

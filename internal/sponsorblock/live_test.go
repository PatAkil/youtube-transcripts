package sponsorblock

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestLiveFetchSlots reaches real SponsorBlock. Run with SPONSORBLOCK_LIVE=1.
func TestLiveFetchSlots(t *testing.T) {
	if os.Getenv("SPONSORBLOCK_LIVE") != "1" {
		t.Skip("set SPONSORBLOCK_LIVE=1 to run the live SponsorBlock smoke test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := NewClient(&http.Client{Timeout: 30 * time.Second})

	// "Me at the zoo" has no sponsor read, so SponsorBlock answers 404 for it:
	// the smoke test checks the round trip and the "no data" path without
	// depending on volunteer submissions that may change.
	windows, err := c.FetchSlots(ctx, "jNQXAC9IVRw")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range windows {
		if w.ID == "" || w.End < w.Start {
			t.Errorf("malformed window %+v", w)
		}
	}
	t.Logf("SponsorBlock returned %d sponsor windows", len(windows))
}

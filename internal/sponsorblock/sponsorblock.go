// Package sponsorblock is the single seam through which the CLI reaches
// SponsorBlock. It exposes a Client interface, a key-free HTTP implementation
// and a map-backed Fake for tests.
package sponsorblock

import "context"

// Window is one SponsorBlock segment of category "sponsor": where it starts
// and ends in seconds, and SponsorBlock's UUID for it. Overlapping or touching
// Windows are merged into one Sponsor Slot by the corpus package.
type Window struct {
	Start float64
	End   float64
	ID    string
}

// Client is the SponsorBlock seam. FetchSlots returns the Video's
// sponsor-category Windows; a Video SponsorBlock has no data on yields an
// empty list and no error.
type Client interface {
	FetchSlots(ctx context.Context, videoID string) ([]Window, error)
}

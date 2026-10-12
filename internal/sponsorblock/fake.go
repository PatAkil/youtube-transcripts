package sponsorblock

import "context"

// Fake is an in-memory Client keyed by video ID. Tests fill Windows with
// canned sponsor Windows; a video ID absent from both maps reads as "no data"
// (an empty list, no error), the way real SponsorBlock answers 404.
type Fake struct {
	Windows map[string][]Window
	// Errors makes FetchSlots fail for the given video ID.
	Errors map[string]error
	// Calls counts every FetchSlots call, so a test can assert that
	// SponsorBlock was not contacted.
	Calls int
}

var _ Client = (*Fake)(nil)

// FetchSlots returns the canned Windows for videoID.
func (f *Fake) FetchSlots(_ context.Context, videoID string) ([]Window, error) {
	f.Calls++
	if err, ok := f.Errors[videoID]; ok {
		return nil, err
	}
	return f.Windows[videoID], nil
}

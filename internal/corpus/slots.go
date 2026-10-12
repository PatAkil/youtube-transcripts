package corpus

import (
	"slices"

	"github.com/PatAkil/youtube-transcripts/internal/sponsorblock"
)

// SlotsFile holds a Video's Sponsor Slots. It is present even when the list
// is empty, so "SponsorBlock has no data" is recorded, not merely absent.
const SlotsFile = "slots.json"

// Slot is one Sponsor Slot: a contiguous window of the Video given over to
// sponsor reads, in seconds, with the SponsorBlock segment IDs it was merged
// from. A Slot may hold several Sponsor Segments; the agent pass splits them.
type Slot struct {
	Start     float64  `json:"start"`
	End       float64  `json:"end"`
	SourceIDs []string `json:"source_ids"`
}

// Slots is the slots.json file.
type Slots struct {
	Slots []Slot `json:"slots"`
}

// MergeWindows turns SponsorBlock Windows into Slots sorted by start: Windows
// that overlap or touch are merged into one Slot, with every source ID kept
// in start order. The input is not modified. The result is never nil so it
// marshals as an empty list.
func MergeWindows(windows []sponsorblock.Window) []Slot {
	sorted := slices.Clone(windows)
	slices.SortStableFunc(sorted, func(a, b sponsorblock.Window) int {
		switch {
		case a.Start < b.Start:
			return -1
		case a.Start > b.Start:
			return 1
		}
		return 0
	})
	slots := make([]Slot, 0, len(sorted))
	for _, w := range sorted {
		if n := len(slots); n > 0 && w.Start <= slots[n-1].End {
			last := &slots[n-1]
			if w.End > last.End {
				last.End = w.End
			}
			last.SourceIDs = append(last.SourceIDs, w.ID)
			continue
		}
		slots = append(slots, Slot{Start: w.Start, End: w.End, SourceIDs: []string{w.ID}})
	}
	return slots
}

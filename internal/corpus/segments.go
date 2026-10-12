package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Segment origins: where the agent pass found the read.
const (
	OriginSlot       = "slot"       // inside a Sponsor Slot from SponsorBlock
	OriginTranscript = "transcript" // found by reading the Transcript itself
)

// Segments is the segments.json file: the agent pass's verdict on one Video.
// An empty Segments list with a Processed time means "processed, no Sponsors
// found". The file is written only by the agent pass; the CLI reads it.
// The schema is documented in docs/segments-file.md.
type Segments struct {
	Processed time.Time `json:"processed"`
	Segments  []Segment `json:"segments"`
}

// Segment is one Sponsor's paid read within a Video.
type Segment struct {
	Sponsor string  `json:"sponsor"`
	Start   float64 `json:"start"` // seconds
	End     float64 `json:"end"`   // seconds
	Origin  string  `json:"origin"`
	// SlotIndex is the index into slots.json when Origin is OriginSlot, and
	// nil (omitted) when Origin is OriginTranscript.
	SlotIndex   *int   `json:"slot_index,omitempty"`
	OpeningLine string `json:"opening_line"`
}

// ReadSegments parses and validates a segments.json file.
func ReadSegments(path string) (Segments, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Segments{}, err
	}
	var raw struct {
		Processed *time.Time `json:"processed"`
		Segments  []Segment  `json:"segments"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Segments{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if raw.Processed == nil || raw.Processed.IsZero() {
		return Segments{}, fmt.Errorf("%s: missing processed timestamp", path)
	}
	for i, s := range raw.Segments {
		if err := s.validate(); err != nil {
			return Segments{}, fmt.Errorf("%s: segment %d: %w", path, i, err)
		}
	}
	segs := raw.Segments
	if segs == nil {
		segs = []Segment{}
	}
	return Segments{Processed: *raw.Processed, Segments: segs}, nil
}

func (s Segment) validate() error {
	if strings.TrimSpace(s.Sponsor) == "" {
		return fmt.Errorf("sponsor name is empty")
	}
	if s.Start < 0 {
		return fmt.Errorf("start %v is negative", s.Start)
	}
	if s.End < s.Start {
		return fmt.Errorf("end %v is before start %v", s.End, s.Start)
	}
	switch s.Origin {
	case OriginSlot:
		if s.SlotIndex == nil {
			return fmt.Errorf("origin %q needs a slot_index", s.Origin)
		}
		if *s.SlotIndex < 0 {
			return fmt.Errorf("slot_index %d is negative", *s.SlotIndex)
		}
	case OriginTranscript:
	default:
		return fmt.Errorf("unknown origin %q (want %q or %q)", s.Origin, OriginSlot, OriginTranscript)
	}
	return nil
}

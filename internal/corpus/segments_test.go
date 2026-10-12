package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeSegmentsFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), SegmentsFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadSegmentsParsesSlotAndTranscriptSegments(t *testing.T) {
	path := writeSegmentsFixture(t, `{
  "processed": "2026-10-12T09:30:00Z",
  "segments": [
    {"sponsor": "Acme", "start": 12.5, "end": 60.0, "origin": "slot", "slot_index": 0, "opening_line": "This video is sponsored by Acme."},
    {"sponsor": "Globex", "start": 600, "end": 655.25, "origin": "transcript", "opening_line": "Thanks to Globex for supporting the channel."}
  ]
}
`)
	got, err := ReadSegments(path)
	if err != nil {
		t.Fatalf("ReadSegments: %v", err)
	}
	if want := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC); !got.Processed.Equal(want) {
		t.Errorf("Processed = %v, want %v", got.Processed, want)
	}
	if len(got.Segments) != 2 {
		t.Fatalf("got %d segments, want 2", len(got.Segments))
	}
	first := got.Segments[0]
	if first.Sponsor != "Acme" || first.Start != 12.5 || first.End != 60 || first.Origin != OriginSlot || first.SlotIndex == nil || *first.SlotIndex != 0 || first.OpeningLine != "This video is sponsored by Acme." {
		t.Errorf("first segment = %+v", first)
	}
	second := got.Segments[1]
	if second.Sponsor != "Globex" || second.Start != 600 || second.End != 655.25 || second.Origin != OriginTranscript || second.SlotIndex != nil {
		t.Errorf("second segment = %+v", second)
	}
}

func TestReadSegmentsAcceptsProcessedWithNoSponsorsFound(t *testing.T) {
	path := writeSegmentsFixture(t, `{"processed": "2026-10-12T09:30:00Z", "segments": []}`)
	got, err := ReadSegments(path)
	if err != nil {
		t.Fatalf("ReadSegments: %v", err)
	}
	if len(got.Segments) != 0 || got.Processed.IsZero() {
		t.Errorf("got %+v, want processed with no segments", got)
	}
}

func TestReadSegmentsRejectsInvalidFiles(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"not json", `{"processed": `, "parse"},
		{"missing processed", `{"segments": []}`, "processed"},
		{"unknown origin", `{"processed": "2026-10-12T09:30:00Z", "segments": [{"sponsor": "Acme", "start": 1, "end": 2, "origin": "guess"}]}`, "origin"},
		{"negative start", `{"processed": "2026-10-12T09:30:00Z", "segments": [{"sponsor": "Acme", "start": -1, "end": 2, "origin": "transcript"}]}`, "start"},
		{"end before start", `{"processed": "2026-10-12T09:30:00Z", "segments": [{"sponsor": "Acme", "start": 5, "end": 2, "origin": "transcript"}]}`, "end"},
		{"blank sponsor", `{"processed": "2026-10-12T09:30:00Z", "segments": [{"sponsor": "  ", "start": 1, "end": 2, "origin": "transcript"}]}`, "sponsor"},
		{"slot origin without slot index", `{"processed": "2026-10-12T09:30:00Z", "segments": [{"sponsor": "Acme", "start": 1, "end": 2, "origin": "slot"}]}`, "slot_index"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSegmentsFixture(t, tt.body)
			_, err := ReadSegments(path)
			if err == nil {
				t.Fatal("ReadSegments returned no error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

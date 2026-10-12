package corpus

import (
	"reflect"
	"testing"

	"github.com/PatAkil/youtube-transcripts/internal/sponsorblock"
)

func TestMergeWindows(t *testing.T) {
	tests := []struct {
		name string
		in   []sponsorblock.Window
		want []Slot
	}{
		{
			name: "empty input is an empty list, not nil",
			in:   nil,
			want: []Slot{},
		},
		{
			name: "non-overlapping windows stay separate",
			in: []sponsorblock.Window{
				{Start: 10, End: 20, ID: "a"},
				{Start: 30, End: 40, ID: "b"},
			},
			want: []Slot{
				{Start: 10, End: 20, SourceIDs: []string{"a"}},
				{Start: 30, End: 40, SourceIDs: []string{"b"}},
			},
		},
		{
			name: "overlapping windows merge into one Slot keeping every source ID",
			in: []sponsorblock.Window{
				{Start: 12.5, End: 60, ID: "a"},
				{Start: 45, End: 87, ID: "b"},
			},
			want: []Slot{
				{Start: 12.5, End: 87, SourceIDs: []string{"a", "b"}},
			},
		},
		{
			name: "touching windows merge into one Slot",
			in: []sponsorblock.Window{
				{Start: 10, End: 20, ID: "a"},
				{Start: 20, End: 30, ID: "b"},
			},
			want: []Slot{
				{Start: 10, End: 30, SourceIDs: []string{"a", "b"}},
			},
		},
		{
			name: "unsorted input is sorted by start before merging",
			in: []sponsorblock.Window{
				{Start: 300, End: 330, ID: "c"},
				{Start: 45, End: 87, ID: "b"},
				{Start: 12.5, End: 60, ID: "a"},
			},
			want: []Slot{
				{Start: 12.5, End: 87, SourceIDs: []string{"a", "b"}},
				{Start: 300, End: 330, SourceIDs: []string{"c"}},
			},
		},
		{
			name: "a window inside another does not shrink or extend the Slot",
			in: []sponsorblock.Window{
				{Start: 10, End: 100, ID: "outer"},
				{Start: 20, End: 30, ID: "inner"},
				{Start: 90, End: 95, ID: "late"},
			},
			want: []Slot{
				{Start: 10, End: 100, SourceIDs: []string{"outer", "inner", "late"}},
			},
		},
		{
			name: "a gap of a fraction of a second keeps Slots separate",
			in: []sponsorblock.Window{
				{Start: 10, End: 20, ID: "a"},
				{Start: 20.5, End: 30, ID: "b"},
			},
			want: []Slot{
				{Start: 10, End: 20, SourceIDs: []string{"a"}},
				{Start: 20.5, End: 30, SourceIDs: []string{"b"}},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MergeWindows(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("MergeWindows(%+v) =\n%+v\nwant\n%+v", tc.in, got, tc.want)
			}
		})
	}
}

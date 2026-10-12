package sponsorblock

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestFakeReturnsCannedWindowsNoDataAndErrors(t *testing.T) {
	boom := errors.New("sponsorblock: HTTP 500")
	f := &Fake{
		Windows: map[string][]Window{"known": {{Start: 1, End: 2, ID: "a"}}},
		Errors:  map[string]error{"broken": boom},
	}
	ctx := context.Background()

	got, err := f.FetchSlots(ctx, "known")
	if err != nil || !reflect.DeepEqual(got, []Window{{Start: 1, End: 2, ID: "a"}}) {
		t.Errorf("known: got %+v, %v", got, err)
	}

	got, err = f.FetchSlots(ctx, "unknown")
	if err != nil || len(got) != 0 {
		t.Errorf("unknown video must read as no data, got %+v, %v", got, err)
	}

	if _, err = f.FetchSlots(ctx, "broken"); !errors.Is(err, boom) {
		t.Errorf("broken: err = %v, want %v", err, boom)
	}
}

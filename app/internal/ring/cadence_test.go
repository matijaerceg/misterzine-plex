package ring

import "testing"

func TestCadenceTargetEveryField(t *testing.T) {
	for _, tc := range []struct {
		previous, current, want uint32
		missed                  bool
	}{
		{100, 100, 101, false}, {100, 101, 101, false},
		{100, 102, 102, true}, {0xffffffff, 0, 0, false},
		{0xffffffff, 1, 1, true},
	} {
		got, missed := cadenceTarget(tc.previous, tc.current, 1)
		if got != tc.want || missed != tc.missed {
			t.Fatalf("got %d/%v, want %d/%v", got, missed, tc.want, tc.missed)
		}
	}
}

func TestCadenceTargetPreservesParity(t *testing.T) {
	for _, tc := range []struct {
		previous, current, want uint32
		missed                  bool
	}{
		{100, 100, 102, false}, {100, 101, 102, false}, {100, 102, 102, false},
		{100, 103, 104, true}, {100, 104, 104, true}, {100, 105, 106, true},
		{0xfffffffe, 0xffffffff, 0, false}, {0xfffffffe, 0, 0, false},
		{0xfffffffe, 1, 2, true}, {0xffffffff, 0, 1, false},
	} {
		got, missed := cadenceTarget(tc.previous, tc.current, 2)
		if got != tc.want || missed != tc.missed {
			t.Fatalf("previous=%d current=%d: got %d/%v, want %d/%v", tc.previous, tc.current, got, missed, tc.want, tc.missed)
		}
	}
}

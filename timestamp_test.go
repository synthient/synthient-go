package synthient

import (
	"testing"
	"time"
)

func TestTimestampTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   Timestamp
		want time.Time
	}{
		{in: 0, want: time.Time{}},
		{in: 1714780800, want: time.Date(2024, 5, 4, 0, 0, 0, 0, time.UTC)},
		{in: 1778200137487, want: time.UnixMilli(1778200137487).UTC()},
	}
	for _, tt := range tests {
		got := tt.in.Time()
		if !got.Equal(tt.want) || got.Location() != tt.want.Location() {
			t.Errorf("Timestamp(%d).Time() = %v, want %v", tt.in, got, tt.want)
		}
	}
}

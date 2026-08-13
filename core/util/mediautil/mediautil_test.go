package mediautil

import "testing"

func TestThumbnailTimestamp(t *testing.T) {
	tests := []struct {
		duration int
		want     float64
	}{
		{duration: 0, want: 0},
		{duration: 5, want: 2.5},
		{duration: 60, want: 10},
		{duration: 300, want: 30},
		{duration: 1200, want: 60},
	}

	for _, test := range tests {
		if got := thumbnailTimestamp(test.duration); got != test.want {
			t.Errorf("thumbnailTimestamp(%d) = %v, want %v", test.duration, got, test.want)
		}
	}
}

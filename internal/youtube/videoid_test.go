package youtube

import "testing"

func TestParseVideoID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bare id", "dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"bare id with surrounding whitespace", "  dQw4w9WgXcQ\n", "dQw4w9WgXcQ"},
		{"watch url", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"watch url with extra params", "https://www.youtube.com/watch?t=42&v=dQw4w9WgXcQ&list=PL123", "dQw4w9WgXcQ"},
		{"watch url without scheme", "youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"mobile watch url", "https://m.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"youtu.be", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"youtu.be with params", "https://youtu.be/dQw4w9WgXcQ?si=abcdef&t=10", "dQw4w9WgXcQ"},
		{"shorts", "https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"shorts with params", "https://youtube.com/shorts/dQw4w9WgXcQ?feature=share", "dQw4w9WgXcQ"},
		{"embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embed with params", "https://www.youtube.com/embed/dQw4w9WgXcQ?autoplay=1", "dQw4w9WgXcQ"},
		{"id with dash and underscore", "https://youtu.be/a-b_c1D2e3F", "a-b_c1D2e3F"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVideoID(tt.input)
			if err != nil {
				t.Fatalf("ParseVideoID(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseVideoID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseVideoIDRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too short", "abc"},
		{"too long", "dQw4w9WgXcQdQw4w9WgXcQ"},
		{"illegal characters", "dQw4w9Wg$cQ"},
		{"watch url without v", "https://www.youtube.com/watch?list=PL123"},
		{"other host", "https://vimeo.com/123456789"},
		{"channel url", "https://www.youtube.com/channel/UC1234567890"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := ParseVideoID(tt.input); err == nil {
				t.Errorf("ParseVideoID(%q) = %q, want error", tt.input, got)
			}
		})
	}
}

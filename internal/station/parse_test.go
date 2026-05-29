package station

import (
	"testing"

	"claudepanel/internal/config"
)

func TestParseItem(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKind config.StationItemKind
		wantID   string
		wantErr  bool
	}{
		{"bare video id", "YmQ7jRgf4f0", config.ItemVideo, "YmQ7jRgf4f0", false},
		{"watch url", "https://www.youtube.com/watch?v=EWrX250Zhko", config.ItemVideo, "EWrX250Zhko", false},
		{"watch url no scheme", "youtube.com/watch?v=EWrX250Zhko", config.ItemVideo, "EWrX250Zhko", false},
		{"youtu.be short", "https://youtu.be/EWrX250Zhko", config.ItemVideo, "EWrX250Zhko", false},
		{"shorts", "https://www.youtube.com/shorts/EWrX250Zhko", config.ItemVideo, "EWrX250Zhko", false},
		{"embed", "https://www.youtube.com/embed/EWrX250Zhko", config.ItemVideo, "EWrX250Zhko", false},
		{"watch with list -> video wins", "https://www.youtube.com/watch?v=EWrX250Zhko&list=PLAbcdEfGhIjKlMnOpQrSt", config.ItemVideo, "EWrX250Zhko", false},
		{"youtu.be with list -> video wins", "https://youtu.be/EWrX250Zhko?list=PLAbcdEfGhIjKlMnOpQrSt", config.ItemVideo, "EWrX250Zhko", false},
		{"playlist url", "https://www.youtube.com/playlist?list=PLAbcdEfGhIjKlMnOpQrSt", config.ItemPlaylist, "PLAbcdEfGhIjKlMnOpQrSt", false},
		{"list-first query", "https://www.youtube.com/watch?list=PLAbcdEfGhIjKlMnOpQrSt", config.ItemPlaylist, "PLAbcdEfGhIjKlMnOpQrSt", false},
		{"whitespace trimmed", "  YmQ7jRgf4f0  ", config.ItemVideo, "YmQ7jRgf4f0", false},
		{"empty", "", "", "", true},
		{"garbage", "hello", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseItem(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseItem(%q) = %+v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseItem(%q) unexpected error: %v", tt.input, err)
			}
			if got.Kind != tt.wantKind {
				t.Errorf("ParseItem(%q) kind = %q, want %q", tt.input, got.Kind, tt.wantKind)
			}
			if got.ID != tt.wantID {
				t.Errorf("ParseItem(%q) id = %q, want %q", tt.input, got.ID, tt.wantID)
			}
		})
	}
}

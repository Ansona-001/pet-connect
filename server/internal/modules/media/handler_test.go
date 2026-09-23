package media

import "testing"

func TestSniffMatches(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		header      []byte
		want        bool
	}{
		{"jpeg magic bytes match", "image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, true},
		{"jpeg magic bytes mismatch", "image/jpeg", []byte("not a jpeg at all"), false},
		{"png magic bytes match", "image/png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, true},
		{"png magic bytes mismatch", "image/png", []byte{0xFF, 0xD8, 0xFF}, false},
		{"webp magic bytes match", "image/webp", append([]byte("RIFF\x00\x00\x00\x00"), []byte("WEBP")...), true},
		{"webp missing WEBP tag", "image/webp", append([]byte("RIFF\x00\x00\x00\x00"), []byte("AVI ")...), false},
		{"webp too short", "image/webp", []byte("RIFF"), false},
		{"mp4 ftyp box matches", "video/mp4", []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p'}, true},
		{"mp4 without ftyp box", "video/mp4", []byte("not an mp4 container"), false},
		// audio/mp4 shares mp4's ISO-BMFF container magic — see
		// sniffMatches' doc comment on why this pair can't be
		// disambiguated from a few leading bytes.
		{"audio/mp4 accepts the same ftyp box as video/mp4", "audio/mp4", []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p'}, true},
		{"quicktime ftyp box matches", "video/quicktime", []byte{0x00, 0x00, 0x00, 0x14, 'f', 't', 'y', 'p'}, true},
		{"quicktime moov atom matches", "video/quicktime", []byte{0x00, 0x00, 0x00, 0x08, 'm', 'o', 'o', 'v'}, true},
		{"quicktime unrecognized atom", "video/quicktime", []byte{0x00, 0x00, 0x00, 0x08, 'z', 'z', 'z', 'z'}, false},
		{"webm/EBML magic matches", "video/webm", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x00}, true},
		{"audio/webm accepts the same EBML magic", "audio/webm", []byte{0x1A, 0x45, 0xDF, 0xA3}, true},
		{"webm magic mismatch", "video/webm", []byte{0x00, 0x00, 0x00, 0x00}, false},
		{"mp3 with ID3 tag", "audio/mpeg", []byte("ID3\x03\x00"), true},
		{"mp3 with bare frame sync", "audio/mpeg", []byte{0xFF, 0xFB, 0x90}, true},
		{"mp3 mismatch", "audio/mpeg", []byte("definitely not mp3"), false},
		{"ogg magic matches", "audio/ogg", []byte("OggS\x00"), true},
		{"ogg magic mismatch", "audio/ogg", []byte{0x00, 0x00, 0x00, 0x00}, false},
		{"unknown declared content type never matches", "application/pdf", []byte("%PDF-1.4"), false},
		{"empty header never matches a non-empty signature", "image/jpeg", []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sniffMatches(tt.contentType, tt.header); got != tt.want {
				t.Errorf("sniffMatches(%q, %v) = %v, want %v", tt.contentType, tt.header, got, tt.want)
			}
		})
	}
}

func TestAcceptedMedia(t *testing.T) {
	tests := []struct {
		contentType   string
		wantMediaType string
		wantOK        bool
	}{
		{"image/jpeg", "image", true},
		{"image/png", "image", true},
		{"image/webp", "image", true},
		{"video/mp4", "video", true},
		{"video/webm", "video", true},
		{"video/quicktime", "video", true},
		{"audio/mpeg", "audio", true},
		{"audio/mp4", "audio", true},
		{"audio/webm", "audio", true},
		{"audio/ogg", "audio", true},
		{"application/pdf", "", false},
		{"text/plain", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			extension, mediaType, ok := acceptedMedia(tt.contentType)
			if ok != tt.wantOK {
				t.Fatalf("acceptedMedia(%q) ok = %v, want %v", tt.contentType, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if mediaType != tt.wantMediaType {
				t.Errorf("acceptedMedia(%q) mediaType = %q, want %q", tt.contentType, mediaType, tt.wantMediaType)
			}
			if extension == "" {
				t.Errorf("acceptedMedia(%q) returned an empty extension", tt.contentType)
			}
		})
	}
}

package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

// markerImage builds a 4x2 (width x height) image with a single white
// marker pixel at its top-left corner (0,0) and black everywhere else —
// small and asymmetric enough (width != height) that every one of the 8
// EXIF orientations moves the marker to a distinct, easily-checked
// position, including the four that swap width and height.
func markerImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.Black)
		}
	}
	img.Set(0, 0, color.White)
	return img
}

func isWhite(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r == 0xFFFF && g == 0xFFFF && b == 0xFFFF
}

func TestApplyOrientation(t *testing.T) {
	tests := []struct {
		orientation              int
		wantWidth, wantHeight    int
		wantMarkerX, wantMarkerY int
	}{
		{1, 4, 2, 0, 0}, // normal: no-op
		{2, 4, 2, 3, 0}, // flip horizontal: top-left -> top-right
		{3, 4, 2, 3, 1}, // rotate 180: top-left -> bottom-right
		{4, 4, 2, 0, 1}, // flip vertical: top-left -> bottom-left
		{5, 2, 4, 0, 0}, // transpose: top-left -> top-left, dimensions swap
		{6, 2, 4, 1, 0}, // rotate 90 CW
		{7, 2, 4, 1, 3}, // transverse
		{8, 2, 4, 0, 3}, // rotate 270 CW
	}

	for _, tt := range tests {
		result := applyOrientation(markerImage(), tt.orientation)
		bounds := result.Bounds()
		if bounds.Dx() != tt.wantWidth || bounds.Dy() != tt.wantHeight {
			t.Errorf("orientation %d: bounds = %dx%d, want %dx%d", tt.orientation, bounds.Dx(), bounds.Dy(), tt.wantWidth, tt.wantHeight)
			continue
		}
		found := false
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				if isWhite(result, x, y) {
					found = true
					if x != tt.wantMarkerX || y != tt.wantMarkerY {
						t.Errorf("orientation %d: marker at (%d,%d), want (%d,%d)", tt.orientation, x, y, tt.wantMarkerX, tt.wantMarkerY)
					}
				}
			}
		}
		if !found {
			t.Errorf("orientation %d: marker pixel not found in result", tt.orientation)
		}
	}
}

func TestApplyOrientationOutOfRangeIsNoOp(t *testing.T) {
	src := markerImage()
	for _, orientation := range []int{0, -1, 9, 100} {
		result := applyOrientation(src, orientation)
		if result != image.Image(src) {
			t.Errorf("orientation %d: expected the same image returned unchanged", orientation)
		}
	}
}

func TestReadOrientationDefaultsToOneWithoutExif(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, markerImage(), nil); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	if got := readOrientation(bytes.NewReader(buf.Bytes())); got != 1 {
		t.Errorf("readOrientation on a plain JPEG with no EXIF = %d, want 1", got)
	}
}

// buildMinimalExifWithOrientation constructs the smallest valid EXIF
// blob (the payload of a JPEG APP1 segment, after its "Exif\x00\x00"
// prefix) carrying exactly one IFD0 entry: the Orientation tag. Byte
// layout, little-endian ("II") TIFF:
//
//	"II" 0x002A                  - TIFF byte-order + magic number
//	0x00000008                   - offset to IFD0 (right after this header)
//	0x0001                       - IFD0 entry count = 1
//	0x0112 0x0003 0x00000001 v.. - tag=Orientation, type=SHORT, count=1, value
//	0x00000000                   - offset to next IFD = none
func buildMinimalExifWithOrientation(t *testing.T, orientation uint16) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	buf.WriteString("Exif\x00\x00")
	buf.WriteString("II")
	_ = binary.Write(buf, binary.LittleEndian, uint16(0x002A))
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0x0112)) // Orientation tag
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))      // type SHORT
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))      // count
	_ = binary.Write(buf, binary.LittleEndian, orientation)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0)) // pad the 4-byte value slot
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // next IFD offset
	return buf.Bytes()
}

// insertExifSegment splices an APP1 (EXIF) marker segment carrying
// exifPayload right after jpegData's SOI marker.
func insertExifSegment(t *testing.T, jpegData, exifPayload []byte) []byte {
	t.Helper()
	if len(jpegData) < 2 || jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		t.Fatalf("not a valid JPEG (missing SOI marker)")
	}
	segmentLen := len(exifPayload) + 2
	var out bytes.Buffer
	out.Write(jpegData[:2])
	out.WriteByte(0xFF)
	out.WriteByte(0xE1)
	_ = binary.Write(&out, binary.BigEndian, uint16(segmentLen))
	out.Write(exifPayload)
	out.Write(jpegData[2:])
	return out.Bytes()
}

func TestProcessImageStripsMetadataAndPreservesContent(t *testing.T) {
	t.Run("PNG stays PNG", func(t *testing.T) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, markerImage()); err != nil {
			t.Fatalf("encode test png: %v", err)
		}
		result, err := processImage("image/png", bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("processImage: %v", err)
		}
		if result.contentType != "image/png" || result.extension != ".png" {
			t.Errorf("contentType/extension = %q/%q, want image/png/.png", result.contentType, result.extension)
		}
		if result.width != 4 || result.height != 2 {
			t.Errorf("dimensions = %dx%d, want 4x2", result.width, result.height)
		}
		decoded, err := png.Decode(bytes.NewReader(result.data))
		if err != nil {
			t.Fatalf("decode re-encoded png: %v", err)
		}
		if !isWhite(decoded, 0, 0) {
			t.Error("marker pixel lost after PNG re-encode")
		}
	})

	t.Run("JPEG stays JPEG and orientation is applied then stripped", func(t *testing.T) {
		exifPayload := buildMinimalExifWithOrientation(t, 6)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, markerImage(), nil); err != nil {
			t.Fatalf("encode test jpeg: %v", err)
		}
		withExif := insertExifSegment(t, buf.Bytes(), exifPayload)

		// Confirm the fixture itself actually carries a readable
		// orientation tag before trusting processImage's behavior on it.
		if got := readOrientation(bytes.NewReader(withExif)); got != 6 {
			t.Fatalf("fixture orientation = %d, want 6 (test fixture is broken, not the code under test)", got)
		}

		result, err := processImage("image/jpeg", bytes.NewReader(withExif))
		if err != nil {
			t.Fatalf("processImage: %v", err)
		}
		if result.contentType != "image/jpeg" {
			t.Errorf("contentType = %q, want image/jpeg", result.contentType)
		}
		// Orientation 6 (rotate 90 CW) swaps width/height: 4x2 -> 2x4.
		if result.width != 2 || result.height != 4 {
			t.Errorf("dimensions = %dx%d, want 2x4 (orientation 6 should rotate)", result.width, result.height)
		}
		if readOrientation(bytes.NewReader(result.data)) != 1 {
			t.Error("re-encoded JPEG still carries a non-default EXIF orientation — metadata was not stripped")
		}
	})

	t.Run("WebP normalizes to JPEG", func(t *testing.T) {
		fixture, err := os.ReadFile("testdata/marker.webp")
		if err != nil {
			t.Fatalf("read webp fixture: %v", err)
		}
		result, err := processImage("image/webp", bytes.NewReader(fixture))
		if err != nil {
			t.Fatalf("processImage: %v", err)
		}
		if result.contentType != "image/jpeg" || result.extension != ".jpg" {
			t.Errorf("contentType/extension = %q/%q, want image/jpeg/.jpg (WebP has no Go encoder, must normalize)", result.contentType, result.extension)
		}
		if result.width != 4 || result.height != 2 {
			t.Errorf("dimensions = %dx%d, want 4x2", result.width, result.height)
		}
	})
}

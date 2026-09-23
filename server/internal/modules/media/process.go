package media

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/webp"
)

// jpegQuality is used for both genuine JPEG re-encodes and the WebP→JPEG
// normalization below — high enough to be visually lossless for typical
// photo content, well short of quality 100's disproportionate size cost.
const jpegQuality = 90

type processedImage struct {
	data        []byte
	contentType string
	extension   string
	width       int
	height      int
}

// processImage decodes an already-validated (magic-byte-checked,
// dimension-checked) image and re-encodes it from scratch. Decoding to
// pixels and re-encoding — rather than storing the uploaded bytes
// as-is — is what actually strips metadata: Go's image encoders have no
// concept of EXIF/ICC/XMP/comment chunks to begin with, so a decode/
// re-encode round-trip discards all of it unconditionally, not just the
// GPS tags ADR 0002 is specifically concerned with. The one piece of
// EXIF worth reading before it's discarded is JPEG's orientation tag:
// most phone camera photos are stored "sideways" with an orientation
// tag telling viewers how to rotate them, so blindly stripping EXIF
// without first applying that rotation to the actual pixels would leave
// every such photo visibly rotated wrong everywhere in the app.
//
// image/webp has no encoder in the Go standard library or
// golang.org/x/image, so WebP uploads are normalized to JPEG — image/png
// uploads stay PNG (to preserve transparency for what's typically
// graphic/screenshot content, not a photo), and everything else
// (image/jpeg, image/webp) becomes JPEG.
func processImage(contentType string, r io.Reader) (processedImage, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return processedImage{}, fmt.Errorf("read image: %w", err)
	}

	var img image.Image
	if contentType == "image/webp" {
		img, err = webp.Decode(bytes.NewReader(raw))
	} else {
		img, _, err = image.Decode(bytes.NewReader(raw))
	}
	if err != nil {
		return processedImage{}, fmt.Errorf("decode image: %w", err)
	}

	orientation := 1
	if contentType == "image/jpeg" {
		orientation = readOrientation(bytes.NewReader(raw))
	}
	img = applyOrientation(img, orientation)

	var buf bytes.Buffer
	var outContentType, outExtension string
	if contentType == "image/png" {
		if err := png.Encode(&buf, img); err != nil {
			return processedImage{}, fmt.Errorf("encode png: %w", err)
		}
		outContentType, outExtension = "image/png", ".png"
	} else {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return processedImage{}, fmt.Errorf("encode jpeg: %w", err)
		}
		outContentType, outExtension = "image/jpeg", ".jpg"
	}

	bounds := img.Bounds()
	return processedImage{
		data:        buf.Bytes(),
		contentType: outContentType,
		extension:   outExtension,
		width:       bounds.Dx(),
		height:      bounds.Dy(),
	}, nil
}

// readOrientation returns the EXIF Orientation tag's value (1-8), or 1
// (identity — "already right-side up") for any file with no readable
// EXIF data, which covers the large fraction of JPEGs with no EXIF at
// all as gracefully as it covers a genuinely corrupt/absent tag.
func readOrientation(r io.Reader) int {
	metadata, err := exif.Decode(r)
	if err != nil {
		return 1
	}
	tag, err := metadata.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	value, err := tag.Int(0)
	if err != nil || value < 1 || value > 8 {
		return 1
	}
	return value
}

// applyOrientation rewrites src's pixels into the upright orientation
// EXIF's Orientation tag describes. Orientations 5-8 swap width and
// height (a 90°/270° rotation), which is why the destination image's
// bounds aren't always src's own bounds. The eight cases and their
// pixel-mapping formulas are the standard EXIF orientation transforms;
// 1 (or anything out of the defined 1-8 range) is a no-op.
func applyOrientation(src image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}

	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	var dst *image.RGBA
	switch orientation {
	case 5, 6, 7, 8:
		dst = image.NewRGBA(image.Rect(0, 0, height, width))
	default:
		dst = image.NewRGBA(image.Rect(0, 0, width, height))
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixel := src.At(bounds.Min.X+x, bounds.Min.Y+y)
			switch orientation {
			case 2: // flip horizontal
				dst.Set(width-1-x, y, pixel)
			case 3: // rotate 180
				dst.Set(width-1-x, height-1-y, pixel)
			case 4: // flip vertical
				dst.Set(x, height-1-y, pixel)
			case 5: // transpose
				dst.Set(y, x, pixel)
			case 6: // rotate 90 clockwise
				dst.Set(height-1-y, x, pixel)
			case 7: // transverse
				dst.Set(height-1-y, width-1-x, pixel)
			case 8: // rotate 270 clockwise (90 counter-clockwise)
				dst.Set(y, width-1-x, pixel)
			}
		}
	}
	return dst
}

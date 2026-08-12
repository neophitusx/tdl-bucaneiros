package uploader

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestEncodeThumbnailProducesBoundedJPEG(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 640, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 640; x++ {
			src.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}

	data, err := encodeThumbnail(resizeThumbnail(src))
	if err != nil {
		t.Fatalf("encode thumbnail: %v", err)
	}
	if len(data) > thumbnailMaxSize {
		t.Fatalf("thumbnail size = %d, want at most %d", len(data), thumbnailMaxSize)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode JPEG thumbnail: %v", err)
	}
	if got := decoded.Bounds().Size(); got.X != 320 || got.Y != 200 {
		t.Fatalf("thumbnail dimensions = %v, want 320x200", got)
	}
}

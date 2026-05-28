package images

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/deepteams/webp"
)

func TestReencodeConvertsContentImageToWebPAndResizes(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2400, 1200))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 80, A: 255})
		}
	}

	var input bytes.Buffer
	if err := jpeg.Encode(&input, source, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode source jpeg: %v", err)
	}

	processed, mimeType, width, height, extension, err := Reencode(bytes.NewReader(input.Bytes()), ContentProfile())
	if err != nil {
		t.Fatalf("reencode: %v", err)
	}

	if mimeType != MIMEWebP {
		t.Fatalf("mime type = %q, want %q", mimeType, MIMEWebP)
	}
	if extension != ".webp" {
		t.Fatalf("extension = %q, want .webp", extension)
	}
	if width != 1920 || height != 960 {
		t.Fatalf("size = %dx%d, want 1920x960", width, height)
	}
	config, err := webp.DecodeConfig(bytes.NewReader(processed))
	if err != nil {
		t.Fatalf("decode webp config: %v", err)
	}
	if config.Width != width || config.Height != height {
		t.Fatalf("decoded size = %dx%d, want %dx%d", config.Width, config.Height, width, height)
	}
}

func TestReencodeConvertsAvatarToBoundedWebP(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 600, 900))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			source.SetRGBA(x, y, color.RGBA{R: 40, G: uint8((x + y) % 255), B: 180, A: 255})
		}
	}

	var input bytes.Buffer
	if err := jpeg.Encode(&input, source, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode source jpeg: %v", err)
	}

	processed, mimeType, width, height, extension, err := Reencode(bytes.NewReader(input.Bytes()), AvatarProfile())
	if err != nil {
		t.Fatalf("reencode: %v", err)
	}

	if mimeType != MIMEWebP || extension != ".webp" {
		t.Fatalf("output = %q %q, want image/webp .webp", mimeType, extension)
	}
	if width != 341 || height != 512 {
		t.Fatalf("size = %dx%d, want 341x512", width, height)
	}
	if _, err := webp.DecodeConfig(bytes.NewReader(processed)); err != nil {
		t.Fatalf("decode avatar webp config: %v", err)
	}
}

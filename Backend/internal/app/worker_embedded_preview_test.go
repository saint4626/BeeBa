package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"beeba.org/internal/security/images"
)

func TestReadEmbeddedPreviewImageAcceptsBasisPNGBase64(t *testing.T) {
	var input bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(1, 0, color.NRGBA{G: 255, A: 128})
	img.Set(0, 1, color.NRGBA{B: 255, A: 64})
	img.Set(1, 1, color.NRGBA{A: 0})
	if err := png.Encode(&input, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	metadata, err := readEmbeddedPreviewImage(base64.StdEncoding.EncodeToString(input.Bytes()), int64(input.Len()))
	if err != nil {
		t.Fatalf("readEmbeddedPreviewImage returned error: %v", err)
	}
	if metadata.MIMEType != images.MIMEPNG {
		t.Fatalf("mime = %q, want %q", metadata.MIMEType, images.MIMEPNG)
	}
	if metadata.Width != 2 || metadata.Height != 2 {
		t.Fatalf("dimensions = %dx%d, want 2x2", metadata.Width, metadata.Height)
	}
}

func TestReadEmbeddedPreviewImageRejectsOversizedPreview(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString([]byte("not an image"))

	_, err := readEmbeddedPreviewImage(raw, 2)
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("expected maximum size error, got %v", err)
	}
}

func TestNormalizeEmbeddedPreviewBackfillLimit(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "default", input: 0, want: 100},
		{name: "negative", input: -5, want: 100},
		{name: "custom", input: 25, want: 25},
		{name: "cap", input: 2000, want: 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeEmbeddedPreviewBackfillLimit(tt.input); got != tt.want {
				t.Fatalf("normalizeEmbeddedPreviewBackfillLimit(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

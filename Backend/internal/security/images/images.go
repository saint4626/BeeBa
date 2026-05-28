package images

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"math"

	_ "image/jpeg"
	_ "image/png"

	"github.com/deepteams/webp"
	xdraw "golang.org/x/image/draw"
)

const (
	MIMEJPEG = "image/jpeg"
	MIMEPNG  = "image/png"
	MIMEWebP = "image/webp"
)

type ProcessingProfile struct {
	MaxWidth  int
	MaxHeight int
	Quality   float32
	Method    int
	Preset    webp.Preset
}

type Metadata struct {
	Width       int
	Height      int
	MIMEType    string
	Format      string
	SHA256      string
	Bytes       []byte
	DecodedSize int64
}

func ReadAndValidate(reader io.Reader, maxBytes int64) (Metadata, error) {
	if maxBytes <= 0 {
		return Metadata{}, fmt.Errorf("max image size is invalid")
	}
	limited := io.LimitReader(reader, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Metadata{}, fmt.Errorf("read image: %w", err)
	}
	if len(data) == 0 {
		return Metadata{}, fmt.Errorf("image must not be empty")
	}
	if int64(len(data)) > maxBytes {
		return Metadata{}, fmt.Errorf("image exceeds maximum size")
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Metadata{}, fmt.Errorf("image is not a valid JPEG or PNG")
	}
	if config.Width <= 0 || config.Height <= 0 {
		return Metadata{}, fmt.Errorf("image dimensions are invalid")
	}
	if config.Width > 8192 || config.Height > 8192 {
		return Metadata{}, fmt.Errorf("image dimensions exceed 8192px")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return Metadata{}, fmt.Errorf("image is not a valid JPEG or PNG")
	}

	mimeType, err := mimeTypeForFormat(format)
	if err != nil {
		return Metadata{}, err
	}
	sum := sha256.Sum256(data)
	return Metadata{
		Width:       config.Width,
		Height:      config.Height,
		MIMEType:    mimeType,
		Format:      format,
		SHA256:      hex.EncodeToString(sum[:]),
		Bytes:       data,
		DecodedSize: int64(len(data)),
	}, nil
}

func AvatarProfile() ProcessingProfile {
	return ProcessingProfile{
		MaxWidth:  512,
		MaxHeight: 512,
		Quality:   84,
		Method:    4,
		Preset:    webp.PresetPhoto,
	}
}

func ContentProfile() ProcessingProfile {
	return ProcessingProfile{
		MaxWidth:  1920,
		MaxHeight: 1920,
		Quality:   86,
		Method:    4,
		Preset:    webp.PresetPhoto,
	}
}

func Reencode(reader io.Reader, profile ProcessingProfile) ([]byte, string, int, int, string, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, "", 0, 0, "", fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, "", 0, 0, "", fmt.Errorf("image dimensions are invalid")
	}

	profile = normalizeProfile(profile)
	processed := resizeToFit(img, profile.MaxWidth, profile.MaxHeight)
	processedBounds := processed.Bounds()
	width = processedBounds.Dx()
	height = processedBounds.Dy()

	options := webp.OptionsForPreset(profile.Preset, profile.Quality)
	options.Method = profile.Method
	options.UseSharpYUV = true
	options.AlphaQuality = 95

	var out bytes.Buffer
	if err := webp.Encode(&out, processed, options); err != nil {
		return nil, "", 0, 0, "", fmt.Errorf("encode webp: %w", err)
	}

	return out.Bytes(), MIMEWebP, width, height, ".webp", nil
}

func mimeTypeForFormat(format string) (string, error) {
	switch format {
	case "jpeg":
		return MIMEJPEG, nil
	case "png":
		return MIMEPNG, nil
	default:
		return "", fmt.Errorf("only JPEG and PNG images are accepted")
	}
}

func extensionForFormat(format string) string {
	switch format {
	case "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	default:
		return ""
	}
}

func normalizeProfile(profile ProcessingProfile) ProcessingProfile {
	if profile.MaxWidth <= 0 {
		profile.MaxWidth = 1920
	}
	if profile.MaxHeight <= 0 {
		profile.MaxHeight = profile.MaxWidth
	}
	if profile.Quality <= 0 {
		profile.Quality = 86
	}
	if profile.Method <= 0 {
		profile.Method = 4
	}
	return profile
}

func resizeToFit(img image.Image, maxWidth int, maxHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 || (width <= maxWidth && height <= maxHeight) {
		return img
	}

	scale := math.Min(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	nextWidth := max(1, int(math.Round(float64(width)*scale)))
	nextHeight := max(1, int(math.Round(float64(height)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, nextWidth, nextHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, xdraw.Src, nil)
	return dst
}

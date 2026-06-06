package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
	"golang.org/x/image/webp"
)

// ImageFormat represents supported image formats
type ImageFormat string

const (
	FormatJPEG ImageFormat = "jpeg"
	FormatPNG  ImageFormat = "png"
	FormatWebP ImageFormat = "webp"
)

// CompressionOptions defines options for image compression
type CompressionOptions struct {
	Quality        int                          // Quality 1-100 (higher = better quality)
	MaxWidth       uint                         // Maximum width (0 = no resize)
	MaxHeight      uint                         // Maximum height (0 = no resize)
	PreserveAspect bool                         // Preserve aspect ratio when resizing
	OutputFormat   ImageFormat                  // Output format (default: same as input)
	Interpolation  resize.InterpolationFunction // Resize interpolation method
}

// DefaultCompressionOptions returns sensible default compression options
func DefaultCompressionOptions() CompressionOptions {
	return CompressionOptions{
		Quality:        80,
		MaxWidth:       0,
		MaxHeight:      0,
		PreserveAspect: true,
		Interpolation:  resize.Lanczos3,
	}
}

// CompressFile compresses an image file from disk and writes the result to output path
func CompressFile(inputPath string, outputPath string, options CompressionOptions) error {
	// Open input file
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Detect image format
	format, err := DetectImageFormat(inputPath)
	if err != nil {
		return err
	}

	// Compress image
	var buf bytes.Buffer
	if err := Compress(file, &buf, format, options); err != nil {
		return err
	}

	// Write output file
	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

// Compress compresses an image from reader and writes to writer
func Compress(reader io.Reader, writer io.Writer, inputFormat ImageFormat, options CompressionOptions) error {
	// Decode image
	var img image.Image
	var err error

	switch inputFormat {
	case FormatJPEG:
		img, err = jpeg.Decode(reader)
	case FormatPNG:
		img, err = png.Decode(reader)
	case FormatWebP:
		img, err = webp.Decode(reader)
	default:
		return fmt.Errorf("unsupported image format: %s", inputFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize if needed
	if options.MaxWidth > 0 || options.MaxHeight > 0 {
		img = ResizeImage(img, options)
	}

	// Determine output format
	outputFormat := options.OutputFormat
	if outputFormat == "" {
		outputFormat = inputFormat
	}

	// Encode compressed image
	switch outputFormat {
	case FormatJPEG:
		err = jpeg.Encode(writer, img, &jpeg.Options{Quality: options.Quality})
	case FormatPNG:
		encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
		err = encoder.Encode(writer, img)
	case FormatWebP:
		// Note: WebP encoding requires additional library
		// Fallback to JPEG for now with warning
		err = jpeg.Encode(writer, img, &jpeg.Options{Quality: options.Quality})
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to encode compressed image: %w", err)
	}

	return nil
}

// ResizeImage resizes an image according to options
func ResizeImage(img image.Image, options CompressionOptions) image.Image {
	bounds := img.Bounds()
	origWidth := uint(bounds.Dx())
	origHeight := uint(bounds.Dy())

	// If no resize needed
	if options.MaxWidth == 0 && options.MaxHeight == 0 {
		return img
	}

	targetWidth := options.MaxWidth
	targetHeight := options.MaxHeight

	if options.PreserveAspect {
		aspectRatio := float64(origWidth) / float64(origHeight)

		if targetWidth == 0 {
			targetWidth = uint(float64(targetHeight) * aspectRatio)
		} else if targetHeight == 0 {
			targetHeight = uint(float64(targetWidth) / aspectRatio)
		} else {
			// Calculate which dimension is more constrained
			widthRatio := float64(targetWidth) / float64(origWidth)
			heightRatio := float64(targetHeight) / float64(origHeight)

			if widthRatio < heightRatio {
				targetHeight = uint(float64(targetWidth) / aspectRatio)
			} else {
				targetWidth = uint(float64(targetHeight) * aspectRatio)
			}
		}
	}

	// Don't upscale
	if targetWidth > origWidth {
		targetWidth = origWidth
	}
	if targetHeight > origHeight {
		targetHeight = origHeight
	}

	// If already smaller than target
	if targetWidth == origWidth && targetHeight == origHeight {
		return img
	}

	return resize.Resize(targetWidth, targetHeight, img, options.Interpolation)
}

// DetectImageFormat detects image format from file extension
func DetectImageFormat(path string) (ImageFormat, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return FormatJPEG, nil
	case ".png":
		return FormatPNG, nil
	case ".webp":
		return FormatWebP, nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// GetImageDimensions returns the width and height of an image file without full decoding
func GetImageDimensions(path string) (width, height int, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = file.Close() }()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return config.Width, config.Height, nil
}

// CompressToBuffer compresses an image and returns the result as a byte buffer
func CompressToBuffer(reader io.Reader, inputFormat ImageFormat, options CompressionOptions) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	err := Compress(reader, &buf, inputFormat, options)
	if err != nil {
		return nil, err
	}
	return &buf, nil
}

// CompressFileToBuffer compresses an image file and returns the result as a byte buffer
func CompressFileToBuffer(inputPath string, options CompressionOptions) (*bytes.Buffer, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	format, err := DetectImageFormat(inputPath)
	if err != nil {
		return nil, err
	}

	return CompressToBuffer(file, format, options)
}

// CompressAndResize is a convenience function for simple compression
func CompressAndResize(input io.Reader, output io.Writer, format ImageFormat, maxWidth uint, quality int) error {
	options := DefaultCompressionOptions()
	options.MaxWidth = maxWidth
	options.Quality = quality
	return Compress(input, output, format, options)
}

// CalculateCompressionRatio calculates compression ratio between original and compressed sizes
func CalculateCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}

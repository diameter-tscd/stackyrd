package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nfnt/resize"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

// ImageFormat represents supported image formats
type ImageFormat string

const (
	FormatJPEG ImageFormat = "jpeg"
	FormatPNG  ImageFormat = "png"
	FormatGIF  ImageFormat = "gif"
	FormatBMP  ImageFormat = "bmp"
	FormatTIFF ImageFormat = "tiff"
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

// ImageInfo holds metadata about an image
type ImageInfo struct {
	Format   ImageFormat
	Width    int
	Height   int
	FileSize int64
}

// magic byte patterns for image format detection
var magicBytes = map[string][]byte{
	"jpeg": {0xFF, 0xD8, 0xFF},
	"png":  {0x89, 0x50, 0x4E, 0x47},
	"gif":  {0x47, 0x49, 0x46},
	"bmp":  {0x42, 0x4D},
	"tiff": {0x49, 0x49, 0x2A, 0x00}, // little-endian
	"webp": {0x52, 0x49, 0x46, 0x46}, // RIFF header
}

var tiffBE = []byte{0x4D, 0x4D, 0x00, 0x2A} // big-endian TIFF

// DetectFormatFromBytes detects image format from raw bytes (magic bytes).
// Falls back to extension-based detection if magic bytes are inconclusive.
func DetectFormatFromBytes(data []byte) (ImageFormat, bool) {
	for format, magic := range magicBytes {
		if len(data) >= len(magic) && bytes.Equal(data[:len(magic)], magic) {
			if format == "webp" {
				if len(data) >= 12 && string(data[8:12]) == "WEBP" {
					return FormatWebP, true
				}
				return "", false
			}
			return ImageFormat(format), true
		}
	}
	if len(data) >= len(tiffBE) && bytes.Equal(data[:len(tiffBE)], tiffBE) {
		return FormatTIFF, true
	}
	return "", false
}

// DetectImageFormat detects image format from file extension
func DetectImageFormat(path string) (ImageFormat, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return FormatJPEG, nil
	case ".png":
		return FormatPNG, nil
	case ".gif":
		return FormatGIF, nil
	case ".bmp":
		return FormatBMP, nil
	case ".tif", ".tiff":
		return FormatTIFF, nil
	case ".webp":
		return FormatWebP, nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// IsValidImage checks whether the given byte slice represents a valid image.
// It returns the detected format and nil if valid, or an error if decoding fails.
func IsValidImage(data []byte) (ImageFormat, error) {
	format, ok := DetectFormatFromBytes(data)
	if !ok {
		return "", fmt.Errorf("unable to detect image format from content")
	}
	_, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("invalid %s image: %w", format, err)
	}
	return format, nil
}

// IsValidImageFile checks whether the file at path is a valid image.
func IsValidImageFile(path string) (ImageFormat, error) {
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return IsValidImage(data)
}

// GetImageDimensions returns the width and height of an image file without full decoding
func GetImageDimensions(path string) (width, height int, err error) {
	path = filepath.Clean(path)

	file, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open image file: %w", err)
	}
	defer func() { _ = file.Close() }()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return config.Width, config.Height, nil
}

// GetImageInfo returns comprehensive metadata for an image file.
func GetImageInfo(path string) (*ImageInfo, error) {
	path = filepath.Clean(path)

	format, err := DetectImageFormat(path)
	if err != nil {
		return nil, err
	}

	width, height, err := GetImageDimensions(path)
	if err != nil {
		return nil, err
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &ImageInfo{
		Format:   format,
		Width:    width,
		Height:   height,
		FileSize: fi.Size(),
	}, nil
}

// Cropspec defines a crop rectangle
type Cropspec struct {
	X, Y          int // top-left origin
	Width, Height int // crop dimensions (0 = use image bounds)
}

// Crop cuts a rectangular region from the image.
func Crop(img image.Image, spec Cropspec) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	x := clamp(spec.X, 0, srcW-1)
	y := clamp(spec.Y, 0, srcH-1)
	w := spec.Width
	if w == 0 || x+w > srcW {
		w = srcW - x
	}
	h := spec.Height
	if h == 0 || y+h > srcH {
		h = srcH - y
	}

	cropped := image.NewRGBA(image.Rect(0, 0, w, h))
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			cropped.Set(dx, dy, img.At(x+dx, y+dy))
		}
	}
	return cropped
}

// CenterCrop crops the image to the largest centered square, then
// optionally resizes to the target dimensions.
func CenterCrop(img image.Image, targetWidth, targetHeight uint) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var size int
	var x, y int
	if w < h {
		size = w
		x = 0
		y = (h - size) / 2
	} else {
		size = h
		x = (w - size) / 2
		y = 0
	}

	cropped := Crop(img, Cropspec{X: x, Y: y, Width: size, Height: size})

	if targetWidth > 0 || targetHeight > 0 {
		if targetWidth == 0 {
			targetWidth = targetHeight
		}
		if targetHeight == 0 {
			targetHeight = targetWidth
		}
		return resize.Resize(targetWidth, targetHeight, cropped, resize.Lanczos3)
	}

	return cropped
}

// Thumbnail generates a square thumbnail by center-cropping and resizing.
func Thumbnail(img image.Image, size uint) image.Image {
	return CenterCrop(img, size, size)
}

// ResizeImage resizes an image according to options
func ResizeImage(img image.Image, options CompressionOptions) image.Image {
	bounds := img.Bounds()
	origWidth := uint(bounds.Dx())
	origHeight := uint(bounds.Dy())

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
			widthRatio := float64(targetWidth) / float64(origWidth)
			heightRatio := float64(targetHeight) / float64(origHeight)

			if widthRatio < heightRatio {
				targetHeight = uint(float64(targetWidth) / aspectRatio)
			} else {
				targetWidth = uint(float64(targetHeight) * aspectRatio)
			}
		}
	}

	if targetWidth > origWidth {
		targetWidth = origWidth
	}
	if targetHeight > origHeight {
		targetHeight = origHeight
	}

	if targetWidth == origWidth && targetHeight == origHeight {
		return img
	}

	return resize.Resize(targetWidth, targetHeight, img, options.Interpolation)
}

// decodeImage decodes an image from a reader given its format.
func decodeImage(reader io.Reader, format ImageFormat) (image.Image, error) {
	switch format {
	case FormatJPEG:
		return jpeg.Decode(reader)
	case FormatPNG:
		return png.Decode(reader)
	case FormatGIF:
		return gif.Decode(reader)
	case FormatBMP:
		return bmp.Decode(reader)
	case FormatTIFF:
		return tiff.Decode(reader)
	case FormatWebP:
		return webp.Decode(reader)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}
}

// encodeImage encodes an image to a writer in the specified format.
func encodeImage(writer io.Writer, img image.Image, format ImageFormat, quality int) error {
	switch format {
	case FormatJPEG:
		return jpeg.Encode(writer, img, &jpeg.Options{Quality: quality})
	case FormatPNG:
		encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
		return encoder.Encode(writer, img)
	case FormatGIF:
		return gif.Encode(writer, img, &gif.Options{NumColors: 256})
	case FormatBMP:
		return bmp.Encode(writer, img)
	case FormatTIFF:
		return tiff.Encode(writer, img, &tiff.Options{Compression: tiff.Deflate})
	case FormatWebP:
		return jpeg.Encode(writer, img, &jpeg.Options{Quality: quality})
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// CompressFile compresses an image file from disk and writes the result to output path
func CompressFile(inputPath string, outputPath string, options CompressionOptions) error {
	inputPath = filepath.Clean(inputPath)
	outputPath = filepath.Clean(outputPath)

	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer func() { _ = file.Close() }()

	format, err := DetectImageFormat(inputPath)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := Compress(file, &buf, format, options); err != nil {
		return err
	}

	return os.WriteFile(outputPath, buf.Bytes(), 0600)
}

// Compress compresses an image from reader and writes to writer
func Compress(reader io.Reader, writer io.Writer, inputFormat ImageFormat, options CompressionOptions) error {
	img, err := decodeImage(reader, inputFormat)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	if options.MaxWidth > 0 || options.MaxHeight > 0 {
		img = ResizeImage(img, options)
	}

	outputFormat := options.OutputFormat
	if outputFormat == "" {
		outputFormat = inputFormat
	}

	if err := encodeImage(writer, img, outputFormat, options.Quality); err != nil {
		return fmt.Errorf("failed to encode compressed image: %w", err)
	}

	return nil
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
	inputPath = filepath.Clean(inputPath)

	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
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

// ConvertFormat decodes an image from a reader in the source format and
// encodes it in the target format, writing to the writer.
func ConvertFormat(reader io.Reader, writer io.Writer, sourceFormat, targetFormat ImageFormat) error {
	img, err := decodeImage(reader, sourceFormat)
	if err != nil {
		return fmt.Errorf("failed to decode source image: %w", err)
	}

	var quality int
	switch targetFormat {
	case FormatJPEG, FormatWebP:
		quality = 90
	default:
		quality = 0
	}

	if err := encodeImage(writer, img, targetFormat, quality); err != nil {
		return fmt.Errorf("failed to encode target image: %w", err)
	}

	return nil
}

// OptimizeForWeb resizes the image to at most maxWidth/maxHeight and compresses
// it to the target quality. It is a convenience wrapper around CompressAndResize
// that additionally converts to JPEG by default.
func OptimizeForWeb(input io.Reader, output io.Writer, maxWidth uint, quality int) error {
	options := DefaultCompressionOptions()
	options.MaxWidth = maxWidth
	options.Quality = quality
	options.OutputFormat = FormatJPEG
	return compressWithAutoDetect(input, output, options)
}

// compressWithAutoDetect detects the input format automatically and compresses.
func compressWithAutoDetect(reader io.Reader, writer io.Writer, options CompressionOptions) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	inputFormat, ok := DetectFormatFromBytes(data)
	if !ok {
		return fmt.Errorf("unable to detect image format")
	}

	return Compress(bytes.NewReader(data), writer, inputFormat, options)
}

// BatchProcessOptions configures batch image processing.
type BatchProcessOptions struct {
	Concurrency int    // max concurrent goroutines (default: number of CPUs)
	OutputDir   string // output directory (default: same as input)
	Suffix      string // suffix for output filenames (default: "_processed")
	Overwrite   bool   // overwrite existing output files
	OnProgress  func(path string, err error) // progress callback
}

// BatchResult holds the result of processing a single file in a batch.
type BatchResult struct {
	InputPath  string
	OutputPath string
	Err        error
}

// defaultBatchOptions returns sensible defaults for batch processing.
func defaultBatchOptions() BatchProcessOptions {
	return BatchProcessOptions{
		Concurrency: 4,
		Suffix:      "_processed",
	}
}

// BatchProcess processes multiple image files concurrently.
// Each file is compressed according to the provided options.
func BatchProcess(inputPaths []string, compressOpts CompressionOptions, batchOpts BatchProcessOptions) []BatchResult {
	if batchOpts.Concurrency <= 0 {
		batchOpts.Concurrency = defaultBatchOptions().Concurrency
	}
	if batchOpts.Suffix == "" {
		batchOpts.Suffix = defaultBatchOptions().Suffix
	}

	var mu sync.Mutex
	results := make([]BatchResult, 0, len(inputPaths))
	sem := make(chan struct{}, batchOpts.Concurrency)
	var wg sync.WaitGroup

	for _, inputPath := range inputPaths {
		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer wg.Done()
			defer func() { <-sem }()

			result := BatchResult{InputPath: path}

			outputPath := batchOpts.OutputDir
			if outputPath == "" {
				outputPath = filepath.Dir(path)
			}
			ext := filepath.Ext(path)
			base := strings.TrimSuffix(filepath.Base(path), ext)
			result.OutputPath = filepath.Join(outputPath, base+batchOpts.Suffix+ext)

			if !batchOpts.Overwrite {
				if _, err := os.Stat(result.OutputPath); err == nil {
					result.Err = fmt.Errorf("output file exists: %s", result.OutputPath)
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
					if batchOpts.OnProgress != nil {
						batchOpts.OnProgress(path, result.Err)
					}
					return
				}
			}

			err := CompressFile(path, result.OutputPath, compressOpts)
			result.Err = err

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			if batchOpts.OnProgress != nil {
				batchOpts.OnProgress(path, err)
			}
		}(inputPath)
	}

	wg.Wait()
	return results
}

// Rotate rotates the image 90, 180, or 270 degrees clockwise.
func Rotate(img image.Image, degrees int) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	switch degrees % 360 {
	case 0:
		return img
	case 90:
		rotated := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rotated.Set(h-1-y, x, img.At(x, y))
			}
		}
		return rotated
	case 180:
		rotated := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rotated.Set(w-1-x, h-1-y, img.At(x, y))
			}
		}
		return rotated
	case 270:
		rotated := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rotated.Set(y, w-1-x, img.At(x, y))
			}
		}
		return rotated
	default:
		return img
	}
}

// FlipHorizontally flips the image horizontally (mirror).
func FlipHorizontally(img image.Image) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	flipped := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			flipped.Set(w-1-x, y, img.At(x, y))
		}
	}
	return flipped
}

// FlipVertically flips the image vertically.
func FlipVertically(img image.Image) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	flipped := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			flipped.Set(x, h-1-y, img.At(x, y))
		}
	}
	return flipped
}

func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

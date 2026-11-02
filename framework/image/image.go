/*
|--------------------------------------------------------------------------
| Uploaded Images
|--------------------------------------------------------------------------
|
| An uploaded file is whatever its bytes say, not what its name or
| declared type claims. Sniff reads the bytes and accepts only the raster
| formats the application can decode, so SVG, HTML and executables never
| pass as images.
|
| A small file can still declare a picture of billions of pixels and
| exhaust memory once decoded. Every decode here reads the dimensions
| from the header first and refuses anything over the pixel and byte
| limits before allocating the picture. For an icon file that is the
| header of the embedded picture that gets decoded, not the size its
| directory claims.
|
| Phone cameras store pictures sideways and record the turn in EXIF. The
| turn is applied when pixels are decoded, so pictures and their
| dimensions always come out upright, even once the EXIF is gone.
|
*/

package image

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"

	"github.com/sergeymakinen/go-ico"
	"golang.org/x/image/webp"
)

/*
Errors returned while reading images.

ErrUnsupportedFormat means the bytes are not PNG, JPEG, GIF, WebP or
ICO, or that the format cannot be written. ErrTooLarge means the input is over
the byte limit, ErrTooManyPixels that the picture is over the pixel
limit.
*/
var (
	ErrUnsupportedFormat = errors.New("image: unsupported format")
	ErrTooLarge          = errors.New("image: file too large")
	ErrTooManyPixels     = errors.New("image: too many pixels")
)

/*
Format is a raster image format.
*/
type Format string

/*
The formats images are accepted in. All of them can be decoded; WebP
and ICO cannot be written.
*/
const (
	PNG  Format = "png"
	JPEG Format = "jpeg"
	GIF  Format = "gif"
	WebP Format = "webp"
	ICO  Format = "ico"
)

/*
Default limits: 40 million pixels, the size of a large camera photo,
and 20 MiB of input.
*/
const (
	DefaultMaxPixels = 40_000_000
	DefaultMaxBytes  = 20 << 20
)

/*
sniffLen is how many leading bytes content sniffing looks at.
*/
const sniffLen = 512

/*
Limits bound the work of reading one image. MaxPixels caps width times
height, summed over every frame of an animated GIF; MaxBytes caps the
encoded input. A zero limit takes the default.
*/
type Limits struct {
	MaxPixels int64
	MaxBytes  int64
}

/*
Config is what an image's header says about it, with width and height
as the picture appears once its EXIF orientation is applied.
*/
type Config struct {
	Format Format
	Width  int
	Height int
}

/*
ContentType returns the media type of the format, as in "image/png".
Icons use the "image/x-icon" type browsers expect.
*/
func (f Format) ContentType() string {
	if f == ICO {
		return "image/x-icon"
	}

	return "image/" + string(f)
}

/*
Sniff reads the first bytes of r and returns the image format they
start. It returns ErrUnsupportedFormat for anything else, SVG included.
The bytes it reads are consumed.
*/
func Sniff(r io.Reader) (Format, error) {
	head := make([]byte, sniffLen)

	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("sniff image: %w", err)
	}

	return formatOf(head[:n])
}

/*
DecodeConfig reads the whole image within the byte limit and returns its
header, refusing pictures over the pixel limit.
*/
func DecodeConfig(r io.Reader, limits Limits) (Config, error) {
	in, err := load(r, limits)
	if err != nil {
		return Config{}, err
	}

	return in.cfg, nil
}

/*
Decode reads an image within the limits and returns its picture, turned
upright. For an animated GIF the picture is the first frame; for an icon
it is the largest icon.
*/
func Decode(r io.Reader, limits Limits) (image.Image, Config, error) {
	in, err := load(r, limits)
	if err != nil {
		return nil, Config{}, err
	}

	img, err := in.picture()
	if err != nil {
		return nil, Config{}, err
	}

	return img, in.cfg, nil
}

/*
input is an image read into memory whose header passed the limits.
*/
type input struct {
	data        []byte
	cfg         Config
	orientation orientation
}

/*
load reads the input within the byte limit, decodes its header, checks
the pixel limit and reads the EXIF orientation.
*/
func load(r io.Reader, limits Limits) (input, error) {
	data, err := readLimited(r, limits.maxBytes())
	if err != nil {
		return input{}, err
	}

	cfg, err := readHeader(data)
	if err != nil {
		return input{}, err
	}

	frames := 1
	if cfg.Format == GIF {
		frames = gifFrames(data)
	}

	if int64(cfg.Width)*int64(cfg.Height)*int64(frames) > limits.maxPixels() {
		return input{}, ErrTooManyPixels
	}

	turn := readOrientation(data, cfg.Format)
	if turn.swapsAxes() {
		cfg.Width, cfg.Height = cfg.Height, cfg.Width
	}

	return input{data: data, cfg: cfg, orientation: turn}, nil
}

/*
picture decodes the input and turns it upright.
*/
func (in input) picture() (image.Image, error) {
	img, err := decodePicture(in.data, in.cfg.Format)
	if err != nil {
		return nil, err
	}

	return in.orientation.apply(img), nil
}

/*
readLimited reads all of r, failing with ErrTooLarge past max bytes.
*/
func readLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}

	if int64(len(data)) > maxBytes {
		return nil, ErrTooLarge
	}

	return data, nil
}

/*
formatOf returns the format the bytes start with.
*/
func formatOf(head []byte) (Format, error) {
	switch http.DetectContentType(head) {
	case "image/png":
		return PNG, nil
	case "image/jpeg":
		return JPEG, nil
	case "image/gif":
		return GIF, nil
	case "image/webp":
		return WebP, nil
	case "image/x-icon":
		return ICO, nil
	default:
		return "", ErrUnsupportedFormat
	}
}

/*
readHeader sniffs the format and reads the header with that format's
decoder only, so a file never reaches a decoder for another format.
*/
func readHeader(data []byte) (Config, error) {
	format, err := formatOf(data[:min(len(data), sniffLen)])
	if err != nil {
		return Config{}, err
	}

	var header image.Config

	r := bytes.NewReader(data)

	switch format {
	case PNG:
		header, err = png.DecodeConfig(r)
	case JPEG:
		header, err = jpeg.DecodeConfig(r)
	case GIF:
		header, err = gif.DecodeConfig(r)
	case WebP:
		header, err = webp.DecodeConfig(r)
	case ICO:
		header, err = ico.DecodeConfig(r)
	}

	if err != nil {
		return Config{}, fmt.Errorf("decode %s header: %w", format, err)
	}

	return Config{Format: format, Width: header.Width, Height: header.Height}, nil
}

/*
decodePicture decodes the picture with the decoder of a known format.
*/
func decodePicture(data []byte, format Format) (image.Image, error) {
	var (
		img image.Image
		err error
	)

	r := bytes.NewReader(data)

	switch format {
	case PNG:
		img, err = png.Decode(r)
	case JPEG:
		img, err = jpeg.Decode(r)
	case GIF:
		img, err = gif.Decode(r)
	case WebP:
		img, err = webp.Decode(r)
	case ICO:
		img, err = ico.Decode(r)
	}

	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", format, err)
	}

	return img, nil
}

/*
maxPixels returns the pixel limit, or the default when unset.
*/
func (l Limits) maxPixels() int64 {
	if l.MaxPixels > 0 {
		return l.MaxPixels
	}

	return DefaultMaxPixels
}

/*
maxBytes returns the byte limit, or the default when unset.
*/
func (l Limits) maxBytes() int64 {
	if l.MaxBytes > 0 {
		return l.MaxBytes
	}

	return DefaultMaxBytes
}

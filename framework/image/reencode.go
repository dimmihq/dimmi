/*
|--------------------------------------------------------------------------
| Re-encoding
|--------------------------------------------------------------------------
|
| An uploaded image is never served as it arrived. Decoding it and writing
| a fresh file proves it really is an image and leaves behind everything
| that is not pixels: EXIF with camera serials and GPS positions, XMP,
| comments, and any payload hidden after the image data. The EXIF turn
| is applied first, so a phone photo stays upright without its EXIF.
| Animated GIFs keep their frames and timing.
|
| GIF frames are counted before decoding, because a GIF only declares the
| size of one frame and a small file can hold thousands of them.
|
*/

package image

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
)

/*
jpegQuality is the quality JPEGs are written at: visually lossless for
photos at a fraction of the size of the highest setting.
*/
const jpegQuality = 90

/*
GIF layout, from the GIF89a format: the header with the screen's flags
byte, the two block kinds that may repeat, and the image descriptor with
its own flags byte.
*/
const (
	gifHeaderLen       = 13
	gifScreenFlags     = 10
	gifImageSeparator  = 0x2C
	gifExtension       = 0x21
	gifDescriptorLen   = 10
	gifDescriptorFlags = 9
	gifColorTableFlag  = 0x80
	gifColorTableBits  = 0x07
)

/*
Options configure Reencode. Format is the format to write; empty keeps
the input's format, except WebP and ICO, which are written as PNG
because neither can be written. Limits bound the input as in DecodeConfig.
*/
type Options struct {
	Format Format
	Limits Limits
}

/*
Reencode reads an image within the limits, decodes it and writes it
again without any metadata. It returns the new bytes and their config.
An animated GIF written as GIF keeps every frame; written as another
format it keeps the first.
*/
func Reencode(r io.Reader, opts Options) ([]byte, Config, error) {
	in, err := load(r, opts.Limits)
	if err != nil {
		return nil, Config{}, err
	}

	target := opts.Format
	if target == "" {
		target = writable(in.cfg.Format)
	}

	var out bytes.Buffer
	if err := transcode(&out, in, target); err != nil {
		return nil, Config{}, err
	}

	return out.Bytes(), Config{Format: target, Width: in.cfg.Width, Height: in.cfg.Height}, nil
}

/*
writable returns the format an image is written in by default.
*/
func writable(format Format) Format {
	if format == WebP || format == ICO {
		return PNG
	}

	return format
}

/*
encoder returns the function that writes a picture in a format, or
ErrUnsupportedFormat when the format cannot be written.
*/
func encoder(format Format) (func(io.Writer, image.Image) error, error) {
	switch format {
	case PNG:
		return png.Encode, nil
	case JPEG:
		return func(w io.Writer, img image.Image) error {
			return jpeg.Encode(w, img, &jpeg.Options{Quality: jpegQuality})
		}, nil
	case GIF:
		return func(w io.Writer, img image.Image) error {
			return gif.Encode(w, img, nil)
		}, nil
	case WebP, ICO:
	}

	return nil, ErrUnsupportedFormat
}

/*
transcode decodes the input, turns it upright and writes it in another
format. It checks the target can be written before decoding anything.
A GIF written as GIF keeps all of its frames.
*/
func transcode(w io.Writer, in input, to Format) error {
	encode, err := encoder(to)
	if err != nil {
		return err
	}

	if in.cfg.Format == GIF && to == GIF {
		return copyAnimation(w, in.data)
	}

	img, err := in.picture()
	if err != nil {
		return err
	}

	if err := encode(w, img); err != nil {
		return fmt.Errorf("encode %s: %w", to, err)
	}

	return nil
}

/*
copyAnimation decodes every frame of a GIF and writes them again.
*/
func copyAnimation(w io.Writer, data []byte) error {
	anim, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode gif: %w", err)
	}

	if err := gif.EncodeAll(w, anim); err != nil {
		return fmt.Errorf("encode gif: %w", err)
	}

	return nil
}

/*
gifFrames counts the frames of a GIF by walking its blocks without
decompressing any of them. A truncated or malformed file counts the
frames seen so far, at least one; the decoder rejects it later.
*/
func gifFrames(data []byte) int {
	frames := 0
	pos := gifHeaderLen + colorTableLen(data[gifScreenFlags])

	for pos < len(data) {
		switch data[pos] {
		case gifImageSeparator:
			if pos+gifDescriptorLen > len(data) {
				return max(frames, 1)
			}

			pos += gifDescriptorLen + colorTableLen(data[pos+gifDescriptorFlags]) + 1
			pos = skipSubBlocks(data, pos)
			frames++
		case gifExtension:
			pos = skipSubBlocks(data, pos+2)
		default:
			return max(frames, 1)
		}
	}

	return max(frames, 1)
}

/*
colorTableLen returns the size of the color table a GIF flags byte
announces, or zero when there is none.
*/
func colorTableLen(flags byte) int {
	if flags&gifColorTableFlag == 0 {
		return 0
	}

	return 3 << (flags&gifColorTableBits + 1)
}

/*
skipSubBlocks returns the position after the chain of length-prefixed
data blocks starting at pos and its zero terminator.
*/
func skipSubBlocks(data []byte, pos int) int {
	for pos < len(data) && data[pos] != 0 {
		pos += int(data[pos]) + 1
	}

	return pos + 1
}

package image_test

import (
	"bytes"
	"errors"
	stdimage "image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/dimmihq/dimmi/framework/image"
)

/*
brokenIO fails every read and write, like a dropped connection.
*/
type brokenIO struct{}

/*
Read always fails.
*/
func (brokenIO) Read([]byte) (int, error) {
	return 0, errors.New("connection reset")
}

/*
Write always fails.
*/
func (brokenIO) Write([]byte) (int, error) {
	return 0, errors.New("connection reset")
}

/*
fixture reads a file from testdata.
*/
func fixture(t *testing.T, name string) []byte {
	t.Helper()

	b, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)

	return b
}

/*
encodePNG returns a solid PNG of the given size.
*/
func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := stdimage.NewGray(stdimage.Rect(0, 0, width, height))

	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))

	return out.Bytes()
}

/*
encodeGIF returns an animated GIF with the given number of frames.
*/
func encodeGIF(t *testing.T, size, frames int) []byte {
	t.Helper()

	anim := &gif.GIF{}
	for range frames {
		anim.Image = append(anim.Image, stdimage.NewPaletted(stdimage.Rect(0, 0, size, size), palette.Plan9))
		anim.Delay = append(anim.Delay, 5)
	}

	var out bytes.Buffer
	require.NoError(t, gif.EncodeAll(&out, anim))

	return out.Bytes()
}

func TestSniff(t *testing.T) {
	t.Parallel()

	t.Run("names the format the bytes start with", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			format      image.Format
			contentType string
		}{
			"logo.png":     {image.PNG, "image/png"},
			"photo.jpg":    {image.JPEG, "image/jpeg"},
			"animated.gif": {image.GIF, "image/gif"},
			"logo.webp":    {image.WebP, "image/webp"},
			"favicon.ico":  {image.ICO, "image/x-icon"},
		}

		for name, want := range cases {
			got, err := image.Sniff(bytes.NewReader(fixture(t, name)))
			require.NoError(t, err, name)

			assert.Equal(t, want.format, got, name)
			assert.Equal(t, want.contentType, got.ContentType(), name)
		}
	})

	t.Run("refuses what is not a raster image", func(t *testing.T) {
		t.Parallel()

		inputs := map[string][]byte{
			"svg":        fixture(t, "image.svg"),
			"html":       []byte("<!doctype html><script>alert(1)</script>"),
			"executable": []byte("MZ\x90\x00\x03\x00\x00\x00"),
			"empty":      nil,
			"png name":   []byte("logo.png"),
		}

		for name, in := range inputs {
			_, err := image.Sniff(bytes.NewReader(in))

			require.ErrorIs(t, err, image.ErrUnsupportedFormat, name)
		}
	})

	t.Run("reports a failing reader", func(t *testing.T) {
		t.Parallel()

		_, err := image.Sniff(brokenIO{})

		require.ErrorContains(t, err, "connection reset")
	})
}

func TestDecodeConfig(t *testing.T) {
	t.Parallel()

	t.Run("reads the dimensions of every format", func(t *testing.T) {
		t.Parallel()

		cases := map[string]image.Config{
			"logo.png":     {Format: image.PNG, Width: 20, Height: 10},
			"photo.jpg":    {Format: image.JPEG, Width: 64, Height: 48},
			"animated.gif": {Format: image.GIF, Width: 16, Height: 16},
			"logo.webp":    {Format: image.WebP, Width: 20, Height: 10},
			"favicon.ico":  {Format: image.ICO, Width: 48, Height: 48},
		}

		for name, want := range cases {
			got, err := image.DecodeConfig(bytes.NewReader(fixture(t, name)), image.Limits{})
			require.NoError(t, err, name)

			assert.Equal(t, want, got, name)
		}
	})

	t.Run("refuses a decompression bomb from its header", func(t *testing.T) {
		t.Parallel()

		_, err := image.DecodeConfig(bytes.NewReader(fixture(t, "bomb.png")), image.Limits{})

		require.ErrorIs(t, err, image.ErrTooManyPixels)
	})

	t.Run("counts every frame of an animation", func(t *testing.T) {
		t.Parallel()

		anim := encodeGIF(t, 100, 50)

		_, err := image.DecodeConfig(bytes.NewReader(anim), image.Limits{MaxPixels: 100 * 100 * 49})
		require.ErrorIs(t, err, image.ErrTooManyPixels)

		cfg, err := image.DecodeConfig(bytes.NewReader(anim), image.Limits{MaxPixels: 100 * 100 * 50})
		require.NoError(t, err)
		assert.Equal(t, 100, cfg.Width)
	})

	t.Run("refuses input over the byte limit", func(t *testing.T) {
		t.Parallel()

		_, err := image.DecodeConfig(bytes.NewReader(fixture(t, "photo.jpg")), image.Limits{MaxBytes: 100})

		require.ErrorIs(t, err, image.ErrTooLarge)
	})

	t.Run("refuses a corrupt header", func(t *testing.T) {
		t.Parallel()

		png := fixture(t, "logo.png")

		_, err := image.DecodeConfig(bytes.NewReader(png[:20]), image.Limits{})

		require.ErrorContains(t, err, "decode png header")
	})

	t.Run("refuses svg and reports a failing reader", func(t *testing.T) {
		t.Parallel()

		_, err := image.DecodeConfig(bytes.NewReader(fixture(t, "image.svg")), image.Limits{})
		require.ErrorIs(t, err, image.ErrUnsupportedFormat)

		_, err = image.DecodeConfig(brokenIO{}, image.Limits{})
		require.ErrorContains(t, err, "connection reset")
	})
}

func TestDecode(t *testing.T) {
	t.Parallel()

	t.Run("decodes the picture", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"logo.png", "photo.jpg", "animated.gif", "logo.webp", "favicon.ico"} {
			img, cfg, err := image.Decode(bytes.NewReader(fixture(t, name)), image.Limits{})
			require.NoError(t, err, name)

			assert.Equal(t, cfg.Width, img.Bounds().Dx(), name)
			assert.Equal(t, cfg.Height, img.Bounds().Dy(), name)
		}
	})

	t.Run("refuses a bomb before decoding", func(t *testing.T) {
		t.Parallel()

		_, _, err := image.Decode(bytes.NewReader(fixture(t, "bomb.png")), image.Limits{})

		require.ErrorIs(t, err, image.ErrTooManyPixels)
	})

	t.Run("refuses truncated pixel data", func(t *testing.T) {
		t.Parallel()

		png := encodePNG(t, 64, 64)

		_, _, err := image.Decode(bytes.NewReader(png[:len(png)-20]), image.Limits{})

		require.ErrorContains(t, err, "decode png")
	})
}

func TestReencode(t *testing.T) {
	t.Parallel()

	t.Run("drops metadata and trailing payloads", func(t *testing.T) {
		t.Parallel()

		photo := fixture(t, "photo.jpg")
		require.Contains(t, string(photo), "SERIAL-ABC123")

		out, cfg, err := image.Reencode(bytes.NewReader(photo), image.Options{})
		require.NoError(t, err)

		assert.Equal(t, image.Config{Format: image.JPEG, Width: 64, Height: 48}, cfg)

		for _, leak := range []string{"Exif", "SERIAL-ABC123", "Jane Doe", "<script>"} {
			assert.NotContains(t, string(out), leak)
		}

		got, err := image.DecodeConfig(bytes.NewReader(out), image.Limits{})
		require.NoError(t, err)
		assert.Equal(t, cfg, got)
	})

	t.Run("drops png text chunks", func(t *testing.T) {
		t.Parallel()

		out, cfg, err := image.Reencode(bytes.NewReader(fixture(t, "logo.png")), image.Options{})
		require.NoError(t, err)

		assert.Equal(t, image.PNG, cfg.Format)
		assert.NotContains(t, string(out), "tEXt")
		assert.NotContains(t, string(out), "date:create")
	})

	t.Run("writes webp as png", func(t *testing.T) {
		t.Parallel()

		out, cfg, err := image.Reencode(bytes.NewReader(fixture(t, "logo.webp")), image.Options{})
		require.NoError(t, err)

		assert.Equal(t, image.Config{Format: image.PNG, Width: 20, Height: 10}, cfg)

		format, err := image.Sniff(bytes.NewReader(out))
		require.NoError(t, err)
		assert.Equal(t, image.PNG, format)
	})

	t.Run("writes the largest icon as png", func(t *testing.T) {
		t.Parallel()

		out, cfg, err := image.Reencode(bytes.NewReader(fixture(t, "favicon.ico")), image.Options{})
		require.NoError(t, err)

		assert.Equal(t, image.Config{Format: image.PNG, Width: 48, Height: 48}, cfg)

		img, err := png.Decode(bytes.NewReader(out))
		require.NoError(t, err)
		assert.Equal(t, stdimage.Rect(0, 0, 48, 48), img.Bounds())
	})

	t.Run("converts between formats", func(t *testing.T) {
		t.Parallel()

		for _, target := range []image.Format{image.PNG, image.JPEG, image.GIF} {
			out, cfg, err := image.Reencode(bytes.NewReader(fixture(t, "photo.jpg")), image.Options{Format: target})
			require.NoError(t, err, target)

			format, err := image.Sniff(bytes.NewReader(out))
			require.NoError(t, err)
			assert.Equal(t, target, format)
			assert.Equal(t, target, cfg.Format)
		}
	})

	t.Run("keeps every frame of an animation", func(t *testing.T) {
		t.Parallel()

		out, _, err := image.Reencode(bytes.NewReader(fixture(t, "animated.gif")), image.Options{})
		require.NoError(t, err)

		anim, err := gif.DecodeAll(bytes.NewReader(out))
		require.NoError(t, err)
		assert.Len(t, anim.Image, 3)
		assert.NotContains(t, string(out), "comment")
	})

	t.Run("keeps the first frame in another format", func(t *testing.T) {
		t.Parallel()

		out, cfg, err := image.Reencode(bytes.NewReader(fixture(t, "animated.gif")), image.Options{Format: image.PNG})
		require.NoError(t, err)

		assert.Equal(t, image.PNG, cfg.Format)

		img, err := png.Decode(bytes.NewReader(out))
		require.NoError(t, err)

		r, g, b, _ := img.At(0, 0).RGBA()
		assert.Equal(t, [3]uint32{0xffff, 0, 0}, [3]uint32{r, g, b})
	})

	t.Run("refuses formats it cannot write", func(t *testing.T) {
		t.Parallel()

		for _, target := range []image.Format{image.WebP, image.ICO, "bmp"} {
			_, _, err := image.Reencode(bytes.NewReader(fixture(t, "logo.png")), image.Options{Format: target})

			require.ErrorIs(t, err, image.ErrUnsupportedFormat, target)
		}
	})

	t.Run("refuses bombs, oversized input and broken files", func(t *testing.T) {
		t.Parallel()

		for _, bomb := range []string{"bomb.png", "bomb.ico"} {
			_, _, err := image.Reencode(bytes.NewReader(fixture(t, bomb)), image.Options{})
			require.ErrorIs(t, err, image.ErrTooManyPixels, bomb)
		}

		tiny := image.Options{Limits: image.Limits{MaxBytes: 10}}
		_, _, err := image.Reencode(bytes.NewReader(fixture(t, "logo.png")), tiny)
		require.ErrorIs(t, err, image.ErrTooLarge)

		png := encodePNG(t, 64, 64)
		_, _, err = image.Reencode(bytes.NewReader(png[:len(png)-20]), image.Options{})
		require.ErrorContains(t, err, "decode png")

		anim := fixture(t, "animated.gif")
		_, _, err = image.Reencode(bytes.NewReader(anim[:len(anim)-30]), image.Options{})
		require.ErrorContains(t, err, "decode gif")

		icon := fixture(t, "favicon.ico")
		_, _, err = image.Reencode(bytes.NewReader(icon[:30]), image.Options{})
		require.ErrorContains(t, err, "decode ico header")
	})

	t.Run("reports pictures the target format cannot hold", func(t *testing.T) {
		t.Parallel()

		wide := encodePNG(t, 70000, 1)

		_, _, err := image.Reencode(bytes.NewReader(wide), image.Options{Format: image.JPEG})

		require.ErrorContains(t, err, "encode jpeg")
	})

	t.Run("survives malformed gif block structure", func(t *testing.T) {
		t.Parallel()

		anim := fixture(t, "animated.gif")

		for _, cut := range []int{14, 40, 46, 50} {
			_, _, err := image.Reencode(bytes.NewReader(anim[:cut]), image.Options{})

			require.Error(t, err, cut)
		}

		junk := append(append([]byte{}, anim[:len(anim)-1]...), 0x99)
		_, err := image.DecodeConfig(bytes.NewReader(junk), image.Limits{})
		require.NoError(t, err)
	})
}

func TestOrientation(t *testing.T) {
	t.Parallel()

	files := []string{
		"orientation_1.jpg", "orientation_2.jpg", "orientation_3.jpg", "orientation_4.jpg",
		"orientation_5.jpg", "orientation_6.jpg", "orientation_7.jpg", "orientation_8.jpg",
		"orientation_6.png", "orientation_6.webp",
	}

	t.Run("reports upright dimensions", func(t *testing.T) {
		t.Parallel()

		for _, name := range files {
			cfg, err := image.DecodeConfig(bytes.NewReader(fixture(t, name)), image.Limits{})
			require.NoError(t, err, name)

			assert.Equal(t, [2]int{40, 20}, [2]int{cfg.Width, cfg.Height}, name)
		}
	})

	t.Run("turns pictures upright when decoding", func(t *testing.T) {
		t.Parallel()

		for _, name := range files {
			img, cfg, err := image.Decode(bytes.NewReader(fixture(t, name)), image.Limits{})
			require.NoError(t, err, name)

			assert.Equal(t, stdimage.Rect(0, 0, cfg.Width, cfg.Height), img.Bounds(), name)
			assertUpright(t, img, name)
		}
	})

	t.Run("turns pictures upright before dropping the tag", func(t *testing.T) {
		t.Parallel()

		for _, name := range files {
			out, cfg, err := image.Reencode(bytes.NewReader(fixture(t, name)), image.Options{Format: image.PNG})
			require.NoError(t, err, name)

			assert.Equal(t, image.Config{Format: image.PNG, Width: 40, Height: 20}, cfg, name)
			assert.NotContains(t, string(out), "eXIf", name)

			img, err := png.Decode(bytes.NewReader(out))
			require.NoError(t, err, name)
			assertUpright(t, img, name)
		}
	})
}

/*
assertUpright checks the four colored quarters of the orientation
fixtures: red, green, blue and white from the top left.
*/
func assertUpright(t *testing.T, img stdimage.Image, name string) {
	t.Helper()

	quarters := []struct {
		x, y    int
		r, g, b uint32
	}{
		{5, 5, 0xffff, 0, 0},
		{35, 5, 0, 0xffff, 0},
		{5, 15, 0, 0, 0xffff},
		{35, 15, 0xffff, 0xffff, 0xffff},
	}

	for _, q := range quarters {
		r, g, b, _ := img.At(q.x, q.y).RGBA()

		assert.InDelta(t, q.r, r, 0x1000, "%s red at %d,%d", name, q.x, q.y)
		assert.InDelta(t, q.g, g, 0x1000, "%s green at %d,%d", name, q.x, q.y)
		assert.InDelta(t, q.b, b, 0x1000, "%s blue at %d,%d", name, q.x, q.y)
	}
}

func TestCanvas(t *testing.T) {
	t.Parallel()

	t.Run("draws a card and writes it as png", func(t *testing.T) {
		t.Parallel()

		face, err := image.LoadFont(goregular.TTF, 32)
		require.NoError(t, err)

		logo, _, err := image.Decode(bytes.NewReader(fixture(t, "logo.png")), image.Limits{})
		require.NoError(t, err)

		canvas := image.NewCanvas(200, 100)
		canvas.Fill(color.White, stdimage.Rect(0, 0, 200, 100))
		canvas.DrawImage(logo, stdimage.Rect(10, 10, 50, 30))
		canvas.DrawText(face, "Hi", stdimage.Pt(10, 80), color.Black)

		var out bytes.Buffer
		require.NoError(t, canvas.EncodePNG(&out))

		img, err := png.Decode(&out)
		require.NoError(t, err)

		assert.Equal(t, stdimage.Rect(0, 0, 200, 100), img.Bounds())
		assert.Equal(t, color.RGBAModel.Convert(color.White), color.RGBAModel.Convert(img.At(199, 99)))
		assert.Equal(t, color.RGBA{R: 0, G: 128, B: 0, A: 255}, color.RGBAModel.Convert(img.At(30, 20)))
		assert.True(t, hasDarkPixel(img, stdimage.Rect(10, 50, 60, 85)))
	})

	t.Run("measures text and checks glyphs", func(t *testing.T) {
		t.Parallel()

		face, err := image.LoadFont(goregular.TTF, 20)
		require.NoError(t, err)

		assert.Greater(t, face.Measure("Hello world"), face.Measure("Hello"))
		assert.Zero(t, face.Measure(""))
		assert.Positive(t, face.LineHeight())
		assert.True(t, face.HasGlyph('é'))
		assert.False(t, face.HasGlyph('日'))
	})

	t.Run("refuses a file that is not a font", func(t *testing.T) {
		t.Parallel()

		_, err := image.LoadFont([]byte(strings.Repeat("x", 64)), 12)

		require.ErrorContains(t, err, "parse font")
	})

	t.Run("reports a failing writer", func(t *testing.T) {
		t.Parallel()

		err := image.NewCanvas(4, 4).EncodePNG(brokenIO{})

		require.ErrorContains(t, err, "connection reset")
	})
}

/*
hasDarkPixel reports whether any pixel in the area is close to black.
*/
func hasDarkPixel(img stdimage.Image, area stdimage.Rectangle) bool {
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 0x4000 {
				return true
			}
		}
	}

	return false
}

/*
|--------------------------------------------------------------------------
| Thumbnails
|--------------------------------------------------------------------------
|
| A small square picture for a face or a logo. Whatever was uploaded, a
| wide photo or an animation, comes out as a fresh PNG of one still frame,
| cut from the middle so nothing is stretched. Only pixels survive.
|
*/

package image

import (
	"bytes"
	"fmt"
	"image"
	"io"

	xdraw "golang.org/x/image/draw"
)

/*
Thumbnail decodes the picture within the limits and returns a PNG of the
given size in pixels on each side, with the input's details.
*/
func Thumbnail(r io.Reader, size int, limits Limits) ([]byte, Config, error) {
	picture, cfg, err := Decode(r, limits)
	if err != nil {
		return nil, Config{}, err
	}

	canvas := NewCanvas(size, size)
	xdraw.CatmullRom.Scale(canvas.img, canvas.img.Bounds(), picture, centerSquare(picture.Bounds()), xdraw.Over, nil)

	var out bytes.Buffer
	if err := canvas.EncodePNG(&out); err != nil {
		return nil, Config{}, fmt.Errorf("encode thumbnail: %w", err)
	}

	return out.Bytes(), cfg, nil
}

/*
centerSquare returns the largest square in the middle of the bounds.
*/
func centerSquare(bounds image.Rectangle) image.Rectangle {
	side := min(bounds.Dx(), bounds.Dy())
	x := bounds.Min.X + (bounds.Dx()-side)/2
	y := bounds.Min.Y + (bounds.Dy()-side)/2

	return image.Rect(x, y, x+side, y+side)
}

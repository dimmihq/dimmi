/*
|--------------------------------------------------------------------------
| Drawing
|--------------------------------------------------------------------------
|
| Just enough drawing to lay out a generated card: fill areas with a
| color, place a scaled picture, write text in an embedded TrueType or
| OpenType font, and save the result as PNG. Layout itself, where things
| go and how text wraps, belongs to the caller; a face can measure text
| and say whether it has a glyph so the caller can wrap lines or fall
| back to another font.
|
*/

package image

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

/*
fontDPI makes a font's size in points equal its size in pixels.
*/
const fontDPI = 72

/*
Canvas is a picture to draw on, starting fully transparent. It is not
safe for concurrent use.
*/
type Canvas struct {
	img *image.RGBA
}

/*
Face is a font at one size. It is not safe for concurrent use; load one
per goroutine.
*/
type Face struct {
	face font.Face
	font *sfnt.Font
	buf  sfnt.Buffer
}

/*
NewCanvas returns a transparent canvas of the given size in pixels.
*/
func NewCanvas(width, height int) *Canvas {
	return &Canvas{img: image.NewRGBA(image.Rect(0, 0, width, height))}
}

/*
Fill paints the area with a solid color, replacing what was there.
*/
func (c *Canvas) Fill(col color.Color, area image.Rectangle) {
	draw.Draw(c.img, area, image.NewUniform(col), image.Point{}, draw.Src)
}

/*
DrawImage scales a picture to fill the area and draws it over what is
already there, respecting its transparency. Keep the aspect ratio by
choosing an area of the same proportions.
*/
func (c *Canvas) DrawImage(src image.Image, area image.Rectangle) {
	xdraw.CatmullRom.Scale(c.img, area, src, src.Bounds(), xdraw.Over, nil)
}

/*
DrawText writes a line of text with its baseline starting at the point.
Characters the face has no glyph for are drawn as its missing-glyph box.
*/
func (c *Canvas) DrawText(face *Face, text string, at image.Point, col color.Color) {
	d := font.Drawer{
		Dst:  c.img,
		Src:  image.NewUniform(col),
		Face: face.face,
		Dot:  fixed.P(at.X, at.Y),
	}

	d.DrawString(text)
}

/*
EncodePNG writes the canvas as a PNG.
*/
func (c *Canvas) EncodePNG(w io.Writer) error {
	if err := png.Encode(w, c.img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}

	return nil
}

/*
LoadFont parses a TrueType or OpenType font and returns it at a size in
pixels.
*/
func LoadFont(ttf []byte, size float64) (*Face, error) {
	parsed, err := opentype.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}

	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: fontDPI, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("open font face: %w", err)
	}

	return &Face{face: face, font: parsed}, nil
}

/*
Measure returns the width of a line of text in pixels, rounded up.
*/
func (f *Face) Measure(text string) int {
	return font.MeasureString(f.face, text).Ceil()
}

/*
LineHeight returns the distance between baselines in pixels.
*/
func (f *Face) LineHeight() int {
	return f.face.Metrics().Height.Ceil()
}

/*
HasGlyph reports whether the font draws the character itself rather than
its missing-glyph box.
*/
func (f *Face) HasGlyph(r rune) bool {
	index, err := f.font.GlyphIndex(&f.buf, r)

	return err == nil && index != 0
}

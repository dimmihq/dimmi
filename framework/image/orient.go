/*
|--------------------------------------------------------------------------
| EXIF Orientation
|--------------------------------------------------------------------------
|
| Cameras write pixels in sensor order and record in the EXIF orientation
| tag how the picture must be turned or mirrored to look right. Browsers
| honor the tag, so a photo looks fine until its EXIF is stripped, then
| shows up sideways. The tag is read from JPEG, PNG and WebP files and
| the turn is applied to the pixels; GIF and icon files have no EXIF.
|
*/

package image

import (
	"bytes"
	"image"

	"github.com/bep/imagemeta"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

/*
orientation is the EXIF orientation tag: how the stored picture must be
turned and mirrored to appear upright.
*/
type orientation int

/*
The eight orientations EXIF defines. Turns are clockwise.
*/
const (
	orientNormal orientation = iota + 1
	orientMirror
	orientRotate180
	orientFlip
	orientTranspose
	orientRotate90
	orientTransverse
	orientRotate270
)

/*
orientationTag is the name of the EXIF orientation tag.
*/
const orientationTag = "Orientation"

/*
readOrientation returns the EXIF orientation of an image. A picture
without EXIF, or with EXIF that does not parse, is taken as upright:
its metadata is about to be dropped anyway, so a broken block must not
fail the upload, and the error is discarded.
*/
func readOrientation(data []byte, format Format) orientation {
	source, ok := metadataFormat(format)
	if !ok {
		return orientNormal
	}

	turn := orientNormal

	_, _ = imagemeta.Decode(imagemeta.Options{
		R:           bytes.NewReader(data),
		ImageFormat: source,
		Sources:     imagemeta.EXIF,
		HandleTag: func(tag imagemeta.TagInfo) error {
			if tag.Tag != orientationTag {
				return nil
			}

			if v, ok := tag.Value.(uint16); ok && v >= uint16(orientNormal) && v <= uint16(orientRotate270) {
				turn = orientation(v)
			}

			return imagemeta.ErrStopWalking
		},
	})

	return turn
}

/*
metadataFormat returns the metadata reader's name for a format that can
carry EXIF.
*/
func metadataFormat(format Format) (imagemeta.ImageFormat, bool) {
	switch format {
	case JPEG:
		return imagemeta.JPEG, true
	case PNG:
		return imagemeta.PNG, true
	case WebP:
		return imagemeta.WebP, true
	case GIF, ICO:
	}

	return 0, false
}

/*
swapsAxes reports whether the turn exchanges width and height.
*/
func (o orientation) swapsAxes() bool {
	return o >= orientTranspose
}

/*
apply returns the picture turned upright, or the picture itself when it
already is.
*/
func (o orientation) apply(img image.Image) image.Image {
	if o == orientNormal {
		return img
	}

	src := img.Bounds()

	size := image.Rect(0, 0, src.Dx(), src.Dy())
	if o.swapsAxes() {
		size = image.Rect(0, 0, src.Dy(), src.Dx())
	}

	dst := image.NewRGBA(size)
	xdraw.NearestNeighbor.Transform(dst, o.matrix(src), img, src, xdraw.Src, nil)

	return dst
}

/*
matrix returns the affine map from picture coordinates to upright
coordinates. Each row gives x' or y' as a*x + b*y + c, with x and y
measured from the picture's top-left corner.
*/
func (o orientation) matrix(src image.Rectangle) f64.Aff3 {
	w, h := float64(src.Dx()), float64(src.Dy())

	var m f64.Aff3

	switch o {
	case orientNormal:
		m = f64.Aff3{1, 0, 0, 0, 1, 0}
	case orientMirror:
		m = f64.Aff3{-1, 0, w, 0, 1, 0}
	case orientRotate180:
		m = f64.Aff3{-1, 0, w, 0, -1, h}
	case orientFlip:
		m = f64.Aff3{1, 0, 0, 0, -1, h}
	case orientTranspose:
		m = f64.Aff3{0, 1, 0, 1, 0, 0}
	case orientRotate90:
		m = f64.Aff3{0, -1, h, 1, 0, 0}
	case orientTransverse:
		m = f64.Aff3{0, -1, h, -1, 0, w}
	case orientRotate270:
		m = f64.Aff3{0, 1, 0, -1, 0, w}
	}

	x0, y0 := float64(src.Min.X), float64(src.Min.Y)
	m[2] -= m[0]*x0 + m[1]*y0
	m[5] -= m[3]*x0 + m[4]*y0

	return m
}

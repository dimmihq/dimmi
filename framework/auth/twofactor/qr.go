/*
|--------------------------------------------------------------------------
| QR Codes
|--------------------------------------------------------------------------
|
| Authenticator apps add an account by scanning its otpauth URL from a QR
| code. The code is drawn as SVG, one square per dark module inside the
| quiet zone scanners need, so it stays sharp at any size and needs no
| image file. The modules come from the QR encoder the one-time password
| library already ships: its picture is read back one module at a time.
|
*/

package twofactor

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"github.com/pquerna/otp"
)

/*
ErrQRCode means the encoder drew a picture that is not a QR code, which
only a change in the encoder could cause.
*/
var ErrQRCode = errors.New("twofactor: cannot read qr code modules")

/*
The QR drawing settings: the size in pixels the encoder draws at, which
leaves every module several pixels wide, the modules of white border
scanners need, and the modules across a finder pattern.
*/
const (
	qrPixels    = 512
	qrQuietZone = 4
	finderWidth = 7
)

/*
QRCodeSVG returns the otpauth URL as a QR code drawn in SVG, black on
white.
*/
func QRCodeSVG(otpauthURL string) (string, error) {
	key, err := otp.NewKeyFromURL(otpauthURL)
	if err != nil {
		return "", fmt.Errorf("parse otpauth url: %w", err)
	}

	img, err := key.Image(qrPixels, qrPixels)
	if err != nil {
		return "", fmt.Errorf("draw qr code: %w", err)
	}

	modules, err := readModules(img)
	if err != nil {
		return "", err
	}

	return drawSVG(modules), nil
}

/*
readModules reads the dark modules back from the encoder's picture. The
picture is centered without a border, so its top-left module is the
corner of a finder pattern: the first dark pixel on the diagonal is the
offset, and the dark run from there spans seven modules.
*/
func readModules(img image.Image) ([][]bool, error) {
	width := img.Bounds().Dx()

	offset := 0
	for offset < width/2 && !dark(img, offset, offset) {
		offset++
	}

	run := 0
	for offset+run < width && dark(img, offset+run, offset) {
		run++
	}

	scale := run / finderWidth
	if scale == 0 || run%finderWidth != 0 {
		return nil, ErrQRCode
	}

	size := (width - 2*offset) / scale
	modules := make([][]bool, size)

	for row := range size {
		modules[row] = make([]bool, size)
		for col := range size {
			modules[row][col] = dark(img, offset+col*scale+scale/2, offset+row*scale+scale/2)
		}
	}

	return modules, nil
}

/*
dark reports whether the pixel is closer to black than to white.
*/
func dark(img image.Image, x, y int) bool {
	gray, _ := color.GrayModel.Convert(img.At(x, y)).(color.Gray)

	return gray.Y < 128
}

/*
drawSVG draws the modules as one path of unit squares on a white
square that includes the quiet zone.
*/
func drawSVG(modules [][]bool) string {
	side := strconv.Itoa(len(modules) + 2*qrQuietZone)

	var path strings.Builder

	for row, line := range modules {
		for col, on := range line {
			if on {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", col+qrQuietZone, row+qrQuietZone)
			}
		}
	}

	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ` + side + " " + side +
		`" shape-rendering="crispEdges"><rect width="` + side + `" height="` + side +
		`" fill="#ffffff"/><path fill="#000000" d="` + path.String() + `"/></svg>`
}

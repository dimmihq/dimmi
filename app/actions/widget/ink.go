/*
|--------------------------------------------------------------------------
| Ink
|--------------------------------------------------------------------------
|
| Text on a customer's brand color turns dark or light by the color's
| lightness, so a white header never hides its own title. The threshold
| matches the widget's own, so server and browser agree.
|
*/

package widget

import "strconv"

/*
inkLightSurface is the luminance above which a surface takes dark text.
*/
const inkLightSurface = 0.62

/*
inkOn returns the ink that reads on the color: dark on a light surface,
light on a dark one.
*/
func inkOn(hex string) string {
	if luminance(hex) > inkLightSurface {
		return "dark"
	}

	return "light"
}

/*
luminance returns the color's relative luminance from 0 to 1.
*/
func luminance(hex string) float64 {
	if len(hex) != 7 {
		return 0
	}

	channel := func(at int) float64 {
		value, _ := strconv.ParseUint(hex[at:at+2], 16, 8)

		return float64(value) / 255
	}

	return 0.2126*channel(1) + 0.7152*channel(3) + 0.0722*channel(5)
}

/*
|--------------------------------------------------------------------------
| No Detection
|--------------------------------------------------------------------------
|
| Language models cost memory a small server may not have, and some
| applications already learn the language elsewhere, such as from a
| classifier. This detector keeps detection switched off: it answers
| every text with no language, which every caller must handle anyway.
|
*/

package none

import (
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/i18n"
)

/*
Detector detects nothing.
*/
type Detector struct{}

var _ i18n.Detector = Detector{}

/*
Factory builds the detector for the detector manager. It has no options.
*/
func Factory(string, config.Section) (i18n.Detector, error) {
	return Detector{}, nil
}

/*
Detect returns no language and no confidence.
*/
func (Detector) Detect(string) (string, float64) {
	return "", 0
}

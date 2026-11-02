/*
|--------------------------------------------------------------------------
| Trigram Detection
|--------------------------------------------------------------------------
|
| whatlanggo guesses a language from the script of the text and how often
| its three-letter sequences appear in each language. Its tables are tiny
| and compiled in, so detection costs no extra memory at runtime, at the
| price of weak guesses on very short text; callers judge the confidence.
|
| Languages are named by their two-letter ISO 639-1 code, or by the
| three-letter ISO 639-3 code for the few that have no two-letter one.
|
*/

package whatlang

import (
	"github.com/abadojack/whatlanggo"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/i18n"
)

/*
Detector detects languages with whatlanggo.
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
Detect returns the language of the text and the confidence in it, or ""
and 0 when the text has no letters to judge.
*/
func (Detector) Detect(text string) (string, float64) {
	info := whatlanggo.Detect(text)
	if info.Lang < 0 {
		return "", 0
	}

	if code := info.Lang.Iso6391(); code != "" {
		return code, info.Confidence
	}

	return info.Lang.Iso6393(), info.Confidence
}

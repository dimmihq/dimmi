/*
|--------------------------------------------------------------------------
| Detector Contract
|--------------------------------------------------------------------------
|
| Callers trust a detector's answer or ignore it, so every detector must
| answer in the same terms: a real language tag with a confidence between
| zero and one, or no answer at all. A detector may always abstain, but
| it may never guess wrong on plain text in a common language.
|
|   func TestDetector(t *testing.T) {
|       drivertest.Run(t, newDetector)
|   }
|
*/

package drivertest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"

	"github.com/dimmihq/dimmi/framework/i18n"
)

/*
NewDetector returns a detector to test.
*/
type NewDetector func(t *testing.T) i18n.Detector

/*
samples are plain sentences in common languages, keyed by their tag.
*/
func samples() map[string]string {
	return map[string]string{
		"en": "The weather is lovely today and we are going to the beach with our friends.",
		"fr": "Il fait très beau aujourd'hui et nous allons à la plage avec nos amis.",
		"de": "Das Wetter ist heute wunderschön und wir gehen mit unseren Freunden an den Strand.",
		"es": "Hoy hace muy buen tiempo y vamos a la playa con nuestros amigos.",
	}
}

/*
Run runs the contract against detectors from the constructor.
*/
func Run(t *testing.T, newDetector NewDetector) {
	t.Helper()

	t.Run("names the language of plain text or abstains", func(t *testing.T) {
		t.Parallel()
		detector := newDetector(t)

		for want, text := range samples() {
			got, confidence := detector.Detect(text)
			if got == "" {
				assert.Zero(t, confidence, want)

				continue
			}

			assert.Equal(t, want, got)
			assert.Equal(t, want, language.Make(got).String())
			assert.Greater(t, confidence, 0.0, want)
			assert.LessOrEqual(t, confidence, 1.0, want)
		}
	})

	t.Run("abstains on text without letters", func(t *testing.T) {
		t.Parallel()
		detector := newDetector(t)

		for _, text := range []string{"", "   ", "12345 !?"} {
			got, confidence := detector.Detect(text)

			assert.Empty(t, got, text)
			assert.Zero(t, confidence, text)
		}
	})
}

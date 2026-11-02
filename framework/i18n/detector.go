/*
|--------------------------------------------------------------------------
| Language Detection
|--------------------------------------------------------------------------
|
| Some text arrives without a locale, such as a message typed by someone
| who never chose one. A detector guesses its language from the text
| alone. Guessing costs memory for language models, so detection is
| optional: the none driver never guesses, and callers must always cope
| with no answer.
|
|   detection:
|     default: text
|     detectors:
|       text:
|         driver: whatlang
|
*/

package i18n

import (
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Detector guesses the language of text. It returns the language's tag,
such as "en", and its confidence between 0 and 1, or "" and 0 when it
cannot tell.
*/
type Detector interface {
	/*
		Detect returns the language of the text and the confidence in it.
	*/
	Detect(text string) (string, float64)
}

/*
DetectorConfig lists the named detectors and which one is the default.
*/
type DetectorConfig struct {
	Default   string                    `koanf:"default"`
	Detectors map[string]config.Section `koanf:"detectors"`
}

/*
DetectorManager resolves named detectors.
*/
type DetectorManager struct {
	detectors *manager.Manager[Detector]
}

/*
NewDetectorManager returns a manager for the detectors in the
configuration.
*/
func NewDetectorManager(cfg DetectorConfig) *DetectorManager {
	return &DetectorManager{detectors: manager.New[Detector]("detector", cfg.Detectors, cfg.Default)}
}

/*
Extend registers the factory for a driver name.
*/
func (m *DetectorManager) Extend(driver string, factory manager.Factory[Detector]) {
	m.detectors.Extend(driver, factory)
}

/*
Detector returns the named detector.
*/
func (m *DetectorManager) Detector(name string) (Detector, error) {
	return m.detectors.Instance(name)
}

/*
Default returns the default detector.
*/
func (m *DetectorManager) Default() (Detector, error) {
	return m.detectors.Default()
}

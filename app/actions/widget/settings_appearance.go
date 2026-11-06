/*
|--------------------------------------------------------------------------
| Widget Appearance
|--------------------------------------------------------------------------
|
| A widget stores only what a customer changed; everything else follows the
| defaults, so improving a default reaches every install that never touched
| it. A change merges key by key, and null puts a key back to its default.
| Every value is checked here, because these end up on customers' pages.
|
*/

package widget

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
maxAppearanceBytes caps a widget's stored appearance.
*/
const maxAppearanceBytes = 8 << 10

/*
maxPadding is the most a launcher may sit from the page's edge, in pixels.
*/
const maxPadding = 200

/*
Launcher is how the launcher looks.
*/
type Launcher struct {
	Style        string  `json:"style"`
	Shape        string  `json:"shape"`
	Label        string  `json:"label"`
	Icon         string  `json:"icon"`
	ImageAssetID *string `json:"image_asset_id,omitempty"`
}

/*
Gradient is a background's two colors and its angle in degrees.
*/
type Gradient struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Angle int    `json:"angle"`
}

/*
Background is what fills the home hero and the header: a solid color, a
gradient or an image, softened into the page below it when Fade is set.
*/
type Background struct {
	Type         string    `json:"type"`
	Gradient     *Gradient `json:"gradient,omitempty"`
	ImageAssetID *string   `json:"image_asset_id,omitempty"`
	Fade         bool      `json:"fade"`
}

/*
Appearance is a widget's look with every default filled in.
*/
type Appearance struct {
	PrimaryColor string     `json:"primary_color"`
	HeaderColor  *string    `json:"header_color"`
	ButtonColor  *string    `json:"button_color"`
	LogoAssetID  *string    `json:"logo_asset_id"`
	Background   Background `json:"background"`
	Theme        string     `json:"theme"`
	Position     string     `json:"position"`
	PaddingX     int        `json:"padding_x"`
	PaddingY     int        `json:"padding_y"`
	Font         string     `json:"font"`
	Sound        bool       `json:"sound"`
	Launcher     Launcher   `json:"launcher"`
}

/*
DefaultAppearance is what a widget looks like before anyone changes it.
*/
func DefaultAppearance() Appearance {
	return Appearance{
		PrimaryColor: "#1f2328", Theme: "auto", Position: "right", PaddingX: 20, PaddingY: 20, Font: "system",
		Sound: true, Background: Background{Type: "solid"},
		Launcher: Launcher{Style: "bubble", Shape: "circle", Icon: "chat"},
	}
}

/*
ResolveAppearance returns the stored appearance over the defaults.
*/
func ResolveAppearance(stored json.RawMessage) (Appearance, error) {
	out := DefaultAppearance()
	if len(stored) == 0 {
		return out, nil
	}

	if err := json.Unmarshal(stored, &out); err != nil {
		return out, fmt.Errorf("read widget appearance: %w", err)
	}

	return out, nil
}

/*
isHexColor reports whether the text is a color written #RRGGBB.
*/
func isHexColor(text string) bool {
	digits, ok := strings.CutPrefix(text, "#")
	if !ok || len(digits) != 6 {
		return false
	}

	_, err := strconv.ParseUint(digits, 16, 32)

	return err == nil
}

/*
mergeObject applies a change to a stored object key by key: null removes a
key, and the keys named in nested merge into their own objects.
*/
func mergeObject(stored, change map[string]any, nested ...string) map[string]any {
	out := maps.Clone(stored)
	if out == nil {
		out = map[string]any{}
	}

	for key, value := range change {
		inner, isObject := value.(map[string]any)

		switch {
		case value == nil:
			delete(out, key)
		case isObject && slices.Contains(nested, key):
			current, _ := out[key].(map[string]any)
			out[key] = mergeObject(current, inner)
		default:
			out[key] = value
		}
	}

	return out
}

/*
mergeAppearance returns the stored appearance with the change applied,
refusing a value no widget can show.
*/
func mergeAppearance(stored json.RawMessage, change map[string]any) (json.RawMessage, error) {
	current := map[string]any{}
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &current); err != nil {
			return nil, fmt.Errorf("read widget appearance: %w", err)
		}
	}

	merged := mergeObject(current, change, "launcher", "background")
	if err := checkAppearance(merged); err != nil {
		return nil, err
	}

	out, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("encode widget appearance: %w", err)
	}

	if len(out) > maxAppearanceBytes {
		return nil, exceptions.WidgetField("appearance", "Appearance is too large.")
	}

	return out, nil
}

/*
checkAppearance refuses unknown keys and values out of range.
*/
func checkAppearance(a map[string]any) error {
	checks := map[string]func(any) string{
		"primary_color": requiredColor, "header_color": requiredColor, "button_color": requiredColor,
		"theme": oneOf("light", "dark", "auto"), "position": oneOf("right", "left"),
		"padding_x": between(0, maxPadding), "padding_y": between(0, maxPadding), "font": oneOf("system", "inherit"),
		"sound": isBool, "logo_asset_id": isAssetID,
	}

	for key, value := range a {
		switch key {
		case "launcher":
			if err := checkLauncher(value); err != nil {
				return err
			}
		case "background":
			if err := checkBackground(value); err != nil {
				return err
			}
		default:
			check, known := checks[key]
			if !known {
				return failField("appearance."+key, "This is not an appearance setting.")
			}

			if problem := check(value); problem != "" {
				return failField("appearance."+key, problem)
			}
		}
	}

	return nil
}

func failField(field, message string) error {
	return exceptions.WidgetField(field, message)
}

/*
checkLauncher checks the launcher's keys.
*/
func checkLauncher(value any) error {
	launcher, ok := value.(map[string]any)
	if !ok {
		return failField("appearance.launcher", "Use an object.")
	}

	checks := map[string]func(any) string{
		"style": oneOf("bubble", "tab", "hidden"), "shape": oneOf("circle", "squircle"),
		"label": textUpTo(24), "icon": oneOf("chat", "help", "wave"), "image_asset_id": isAssetID,
	}

	return checkKeys("appearance.launcher.", launcher, checks)
}

/*
checkBackground checks the background's keys.
*/
func checkBackground(value any) error {
	background, ok := value.(map[string]any)
	if !ok {
		return failField("appearance.background", "Use an object.")
	}

	checks := map[string]func(any) string{
		"type": oneOf("solid", "gradient", "image"), "gradient": gradientCheck, "image_asset_id": isAssetID,
		"fade": isBool,
	}

	return checkKeys("appearance.background.", background, checks)
}

/*
checkKeys runs the checks on an object, refusing keys they do not list.
*/
func checkKeys(prefix string, object map[string]any, checks map[string]func(any) string) error {
	for key, value := range object {
		check, known := checks[key]
		if !known {
			return failField(prefix+key, "This is not an appearance setting.")
		}

		if problem := check(value); problem != "" {
			return failField(prefix+key, problem)
		}
	}

	return nil
}

/*
gradientCheck checks a gradient's colors and angle.
*/
func gradientCheck(value any) string {
	gradient, ok := value.(map[string]any)
	if !ok {
		return "Use an object."
	}

	for _, key := range []string{"from", "to"} {
		if problem := requiredColor(gradient[key]); problem != "" {
			return problem
		}
	}

	return between(0, 359)(gradient["angle"])
}

/*
requiredColor checks a color written #RRGGBB. Null never reaches it, since
a null removes the key before any check runs.
*/
func requiredColor(value any) string {
	text, ok := value.(string)
	if !ok || !isHexColor(text) {
		return "Use #RRGGBB."
	}

	return ""
}

/*
oneOf returns the check of a value from a fixed list.
*/
func oneOf(allowed ...string) func(any) string {
	return func(value any) string {
		text, ok := value.(string)
		if !ok || !slices.Contains(allowed, text) {
			return fmt.Sprintf("Use one of %v.", allowed)
		}

		return ""
	}
}

/*
between returns the check of a whole number in a range.
*/
func between(low, high int) func(any) string {
	return func(value any) string {
		number, ok := value.(float64)
		if !ok || number != float64(int(number)) || int(number) < low || int(number) > high {
			return fmt.Sprintf("Use a whole number from %d to %d.", low, high)
		}

		return ""
	}
}

/*
textUpTo returns the check of a text of at most the length.
*/
func textUpTo(limit int) func(any) string {
	return func(value any) string {
		text, ok := value.(string)
		if !ok || len([]rune(text)) > limit {
			return fmt.Sprintf("Use at most %d characters.", limit)
		}

		return ""
	}
}

/*
isBool checks a true or false.
*/
func isBool(value any) string {
	if _, ok := value.(bool); !ok {
		return "Use true or false."
	}

	return ""
}

/*
isAssetID checks an uploaded asset's id. Null never reaches it, since a
null removes the key before any check runs.
*/
func isAssetID(value any) string {
	if _, ok := value.(string); !ok {
		return "Use an uploaded image's id."
	}

	return ""
}

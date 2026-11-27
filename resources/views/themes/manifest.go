/*
|--------------------------------------------------------------------------
| Theme Manifest
|--------------------------------------------------------------------------
|
| theme.json says which version of the theme API a theme is written for,
| its name, the settings it lets a team change without touching code,
| and the outside origins its pages load from on the help center's own
| domain. The API version lets the variables pages receive change in a
| later version without breaking a theme written for this one.
|
*/

package themes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

/*
APIVersion is the theme API this server renders.
*/
const APIVersion = 1

/*
Manifest is a theme's theme.json.
*/
type Manifest struct {
	APIVersion int       `json:"api_version"`
	Name       string    `json:"name"`
	Version    string    `json:"version,omitempty"`
	Author     string    `json:"author,omitempty"`
	Settings   []Setting `json:"settings,omitempty"`
	Origins    Origins   `json:"content_security_policy,omitzero"`
}

/*
Setting is a value a team sets for the theme, read by templates as
settings.<id>. Type is text, textarea, color, checkbox, number, url or
select; select picks from Options.
*/
type Setting struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Label   string   `json:"label"`
	Info    string   `json:"info,omitempty"`
	Default any      `json:"default,omitempty"`
	Options []string `json:"options,omitempty"`
}

/*
Origins are the outside origins a theme's pages load from, per kind,
honored only on the help center's own domain: on the app's /hc/ path a
theme loads nothing from elsewhere.
*/
type Origins struct {
	Scripts []string `json:"script_src,omitempty"`
	Styles  []string `json:"style_src,omitempty"`
	Images  []string `json:"img_src,omitempty"`
	Fonts   []string `json:"font_src,omitempty"`
	Connect []string `json:"connect_src,omitempty"`
	Frames  []string `json:"frame_src,omitempty"`
	Media   []string `json:"media_src,omitempty"`
}

/*
IsZero reports whether the theme names no outside origin.
*/
func (o Origins) IsZero() bool {
	return len(o.Scripts)+len(o.Styles)+len(o.Images)+len(o.Fonts)+len(o.Connect)+len(o.Frames)+len(o.Media) == 0
}

var (
	settingID   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	hexColor    = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	originShape = regexp.MustCompile(`^https://(\*\.)?[a-z0-9]([a-z0-9-]*[a-z0-9])?` +
		`(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+(:[0-9]{1,5})?$`)
)

/*
settingType says whether a theme may declare a setting of the type.
*/
func settingType(kind string) bool {
	switch kind {
	case "text", "textarea", "color", "checkbox", "number", "url", "select":
		return true
	}
	return false
}

/*
parseManifest reads theme.json and lists what is wrong with it.
*/
func parseManifest(body []byte) (Manifest, []Problem) {
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, []Problem{{File: ManifestFile, Message: "theme.json is not valid: " + err.Error()}}
	}
	var messages []string
	if m.APIVersion != APIVersion {
		messages = append(messages, fmt.Sprintf("api_version must be %d", APIVersion))
	}
	if name := strings.TrimSpace(m.Name); name == "" || len(name) > 100 {
		messages = append(messages, "name must be 1 to 100 characters")
	}
	messages = append(messages, checkSettings(m.Settings)...)
	messages = append(messages, checkOrigins(m.Origins)...)
	problems := make([]Problem, 0, len(messages))
	for _, msg := range messages {
		problems = append(problems, Problem{File: ManifestFile, Message: msg})
	}
	return m, problems
}

/*
checkSettings lists what is wrong with the settings a theme declares.
*/
func checkSettings(settings []Setting) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range settings {
		if msg := checkDeclared(s, seen); msg != "" {
			out = append(out, msg)
		}
		seen[s.ID] = true
	}
	return out
}

func checkDeclared(s Setting, seen map[string]bool) string {
	switch {
	case !settingID.MatchString(s.ID):
		return fmt.Sprintf("setting id %q must be lowercase letters, digits and underscores, "+
			"starting with a letter", s.ID)
	case seen[s.ID]:
		return fmt.Sprintf("setting %q is declared twice", s.ID)
	case !settingType(s.Type):
		return fmt.Sprintf("setting %q has type %q; use text, textarea, color, checkbox, number, url or select",
			s.ID, s.Type)
	case s.Type == "select" && len(s.Options) == 0:
		return fmt.Sprintf("setting %q is a select without options", s.ID)
	case s.Default != nil:
		if err := checkSetting(s, s.Default); err != nil {
			return fmt.Sprintf("setting %q: default %v", s.ID, err)
		}
	}
	return ""
}

/*
checkOrigins lists the outside origins that are not https origins.
*/
func checkOrigins(o Origins) []string {
	var out []string
	for _, group := range [][]string{o.Scripts, o.Styles, o.Images, o.Fonts, o.Connect, o.Frames, o.Media} {
		for _, origin := range group {
			if !originShape.MatchString(origin) {
				out = append(out, fmt.Sprintf(
					"content_security_policy: %q must be an https origin, such as https://fonts.example.com", origin))
			}
		}
	}
	return out
}

/*
checkSetting says what is wrong with a value for the setting, nil when
it fits.
*/
func checkSetting(s Setting, v any) error {
	switch s.Type {
	case "checkbox":
		if _, ok := v.(bool); !ok {
			return errors.New("must be true or false")
		}
	case "number":
		if _, ok := v.(float64); !ok {
			return errors.New("must be a number")
		}
	case "color":
		if c, ok := v.(string); !ok || !hexColor.MatchString(c) {
			return errors.New("must be a color such as #10b981")
		}
	case "url":
		return checkURL(v)
	case "select":
		if c, ok := v.(string); !ok || !slices.Contains(s.Options, c) {
			return fmt.Errorf("must be one of %s", strings.Join(s.Options, ", "))
		}
	default:
		if t, ok := v.(string); !ok || len(t) > 10000 {
			return errors.New("must be text of at most 10000 characters")
		}
	}
	return nil
}

func checkURL(v any) error {
	raw, ok := v.(string)
	if !ok {
		return errors.New("must be text")
	}
	if raw == "" || strings.HasPrefix(raw, "/") {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("must be an http(s) address or a path")
	}
	return nil
}

/*
SettingValues returns the theme's settings: each default, replaced by
the team's value where it set a valid one.
*/
func (m Manifest) SettingValues(values map[string]any) map[string]any {
	out := make(map[string]any, len(m.Settings))
	for _, s := range m.Settings {
		out[s.ID] = s.Default
		if v, ok := values[s.ID]; ok && checkSetting(s, v) == nil {
			out[s.ID] = v
		}
	}
	return out
}

/*
CheckSettings lists what is wrong with values a team sets for the
theme's settings.
*/
func (m Manifest) CheckSettings(values map[string]any) []Problem {
	declared := map[string]Setting{}
	for _, s := range m.Settings {
		declared[s.ID] = s
	}
	var problems []Problem
	for id, v := range values {
		s, ok := declared[id]
		if !ok {
			problems = append(problems, Problem{Message: fmt.Sprintf("the theme has no setting %q", id)})
			continue
		}
		if err := checkSetting(s, v); err != nil {
			problems = append(problems, Problem{Message: fmt.Sprintf("setting %q %v", id, err)})
		}
	}
	return problems
}

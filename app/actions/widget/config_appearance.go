/*
|--------------------------------------------------------------------------
| Widget Config Appearance
|--------------------------------------------------------------------------
|
| How a messenger looks in a visitor's browser: its colors, background and
| launcher. Only what paints the widget is here, never an asset id or a
| stored file name, since the config is public and cached by CDNs.
|
*/

package widget

/*
ConfigGradient is a gradient as a page draws it.
*/
type ConfigGradient struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Angle int    `json:"angle"`
}

/*
ConfigBackground is what fills the home hero and header.
*/
type ConfigBackground struct {
	Type     string          `json:"type"`
	Gradient *ConfigGradient `json:"gradient,omitempty"`
	ImageURL *string         `json:"image_url,omitempty"`
	Fade     bool            `json:"fade"`
}

/*
ConfigLauncher is how the launcher looks.
*/
type ConfigLauncher struct {
	Style    string  `json:"style"`
	Shape    string  `json:"shape"`
	Label    string  `json:"label"`
	Icon     string  `json:"icon"`
	ImageURL *string `json:"image_url"`
}

/*
ConfigAppearance is the look, with ids resolved to addresses and the ink
that reads on each brand surface.
*/
type ConfigAppearance struct {
	PrimaryColor string           `json:"primary_color"`
	HeaderColor  string           `json:"header_color"`
	ButtonColor  string           `json:"button_color"`
	HeaderInk    string           `json:"header_ink"`
	ButtonInk    string           `json:"button_ink"`
	Background   ConfigBackground `json:"background"`
	Theme        string           `json:"theme"`
	Position     string           `json:"position"`
	PaddingX     int              `json:"padding_x"`
	PaddingY     int              `json:"padding_y"`
	Launcher     ConfigLauncher   `json:"launcher"`
	Font         string           `json:"font"`
	Sound        bool             `json:"sound"`
}

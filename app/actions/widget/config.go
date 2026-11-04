/*
|--------------------------------------------------------------------------
| Widget Config Shape
|--------------------------------------------------------------------------
|
| The whole of what a widget tells a browser, and how the browser keeps up
| with replies. Keys only ever get added once clients read them, so an older
| messenger keeps working against a newer install.
|
*/

package widget

/*
ConfigRealtime says where live updates come from and how often to poll when
they cannot.
*/
type ConfigRealtime struct {
	URL               string `json:"url"`
	TokenURL          string `json:"token_url"`
	PollingIntervalMS int64  `json:"polling_interval_ms"`
}

/*
ConfigSurface carries the rule for showing the launcher, in the audience
language, for the browser to evaluate on each page.
*/
type ConfigSurface struct {
	Version   int64          `json:"version"`
	Dynamic   bool           `json:"dynamic"`
	Anonymous map[string]any `json:"anonymous"`
}

/*
Config is what a widget tells a browser.
*/
type Config struct {
	Object           string            `json:"object"`
	AppID            string            `json:"app_id"`
	ConfigVersion    int64             `json:"config_version"`
	Company          ConfigCompany     `json:"company"`
	Appearance       ConfigAppearance  `json:"appearance"`
	Texts            map[string]string `json:"texts"`
	Locale           string            `json:"locale"`
	Dir              string            `json:"dir"`
	Locales          []string          `json:"locales"`
	Tabs             []string          `json:"tabs"`
	OpenTo           string            `json:"open_to"`
	ShowTeamAvatars  bool              `json:"show_team_avatars"`
	ShowAvailability bool              `json:"show_availability"`
	CustomerStatus   bool              `json:"customer_status"`
	DelayNotice      bool              `json:"delay_notice"`
	Privacy          ConfigPrivacy     `json:"privacy"`
	HomeLinks        []ConfigHomeLink  `json:"home_links"`
	Surface          *ConfigSurface    `json:"surface,omitempty"`
	Features         ConfigFeatures    `json:"features"`
	Branding         Branding          `json:"branding"`
	Realtime         ConfigRealtime    `json:"realtime"`
	HoneypotField    string            `json:"honeypot_field"`
}

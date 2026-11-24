/*
|--------------------------------------------------------------------------
| AI Decision Model
|--------------------------------------------------------------------------
|
| A decision model answers fixed questions about a piece of text with
| probabilities instead of writing prose. It is configured here, off
| until the operator sets a key.
|
*/

package config

import "time"

/*
AIDecisionsConfig configures the decision model. Clef answers when its
account is set, TypeSafe otherwise; with neither, the workspace's
answer model judges in its place.
*/
type AIDecisionsConfig struct {
	Clef     AIClefConfig     `koanf:"clef"`
	TypeSafe AITypeSafeConfig `koanf:"typesafe"`
}

/*
AIClefConfig is the Cloudflare account Clef runs on: its id, an API
token allowed to run Workers AI, the API base URL, the model (clef or
clef-flash), the timeout of one call and the price of a million input
tokens in US dollars (output is free). Workers AI keeps no request
after answering it and trains on none.
*/
type AIClefConfig struct {
	AccountID         string        `koanf:"account_id"`
	APIToken          string        `koanf:"api_token"                              secret:"true"`
	BaseURL           string        `default:"https://api.cloudflare.com/client/v4" koanf:"base_url"`
	Model             string        `default:"clef"                                 koanf:"model"`
	Timeout           time.Duration `default:"3s"                                   koanf:"timeout"`
	InputPricePerMtok float64       `default:"0.24"                                 koanf:"input_price_per_mtok"`
}

/*
AITypeSafeConfig is the TypeSafe System One account: its key, the API
base URL, the model or alias, the timeout of one call and the price of
a million input tokens in US dollars (output is free).
*/
type AITypeSafeConfig struct {
	APIKey            string        `koanf:"api_key"                      secret:"true"`
	BaseURL           string        `default:"https://api.typesafe.ai/v1" koanf:"base_url"`
	Model             string        `default:"jev-latest"                 koanf:"model"`
	Timeout           time.Duration `default:"3s"                         koanf:"timeout"`
	InputPricePerMtok float64       `default:"0.042"                      koanf:"input_price_per_mtok"`
}

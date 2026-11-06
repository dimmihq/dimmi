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
AIDecisionsConfig configures the decision model.
*/
type AIDecisionsConfig struct {
	TypeSafe AITypeSafeConfig `koanf:"typesafe"`
}

/*
AITypeSafeConfig is the TypeSafe System One account: its key, the API
base URL, the model or alias, the timeout of one call and the price of
a million input tokens in US dollars (output is free).
*/
type AITypeSafeConfig struct {
	APIKey            string        `koanf:"api_key"`
	BaseURL           string        `default:"https://api.typesafe.ai/v1" koanf:"base_url"`
	Model             string        `default:"jev-latest"                 koanf:"model"`
	Timeout           time.Duration `default:"3s"                         koanf:"timeout"`
	InputPricePerMtok float64       `default:"0.042"                      koanf:"input_price_per_mtok"`
}

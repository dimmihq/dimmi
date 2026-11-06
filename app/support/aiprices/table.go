/*
|--------------------------------------------------------------------------
| Price Table
|--------------------------------------------------------------------------
|
| What a million tokens costs per model, kept as a file so prices move
| without a deploy. The table is the honest cost the usage page and
| the cap read; a model it does not know costs nothing but is flagged,
| so "at cost" never quietly lies.
|
*/

package aiprices

import (
	"encoding/json"
	"math/big"
	"strings"
)

/*
Table is one versioned price list.
*/
type Table struct {
	Version string
	Models  []ModelPrice
}

/*
ModelPrice is one model's token prices in USD per million tokens.
*/
type ModelPrice struct {
	Provider           string
	Match              string
	Model              string
	InputPerMtok       string
	CachedInputPerMtok string
	CacheWritePerMtok  string
	OutputPerMtok      string
	Roles              []string
}

/*
tableFile is the YAML shape.
*/
type tableFile struct {
	Version string       `json:"version"`
	Models  []modelEntry `json:"models"`
}

type modelEntry struct {
	Provider           string   `json:"provider"`
	Match              string   `json:"match"`
	Model              string   `json:"model"`
	InputPerMtok       string   `json:"input_per_mtok"`
	CachedInputPerMtok string   `json:"cached_input_per_mtok"`
	CacheWritePerMtok  string   `json:"cache_write_per_mtok"`
	OutputPerMtok      string   `json:"output_per_mtok"`
	Roles              []string `json:"roles"`
}

/*
Parse loads a table from YAML.
*/
func Parse(data []byte) (*Table, error) {
	var f tableFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}

	t := &Table{Version: f.Version}
	for _, m := range f.Models {
		t.Models = append(t.Models, ModelPrice(m))
	}
	return t, nil
}

/*
Find locates the price for a provider+model, exact match first then
longest prefix.
*/
func (t *Table) Find(provider, model string) *ModelPrice {
	var prefix *ModelPrice
	best := 0

	for i := range t.Models {
		p := &t.Models[i]
		if p.Provider != provider {
			continue
		}
		if p.Match == "exact" && p.Model == model {
			return p
		}
		if p.Match == "prefix" && strings.HasPrefix(model, p.Model) && len(p.Model) > best {
			prefix = p
			best = len(p.Model)
		}
	}
	return prefix
}

/*
TokenCounts are the billable counters of one call.
*/
type TokenCounts struct {
	Input, CachedInput, CacheWrite, Output int64
}

/*
Cost computes the micro-USD cost of one usage, nil when the model has
no price.
*/
func (t *Table) Cost(provider, model string, u TokenCounts) *int64 {
	p := t.Find(provider, model)
	if p == nil {
		return nil
	}

	total := new(big.Rat)
	add := func(price string, tokens int64) {
		if price == "" || tokens <= 0 {
			return
		}
		r, ok := new(big.Rat).SetString(price)
		if !ok {
			return
		}
		total.Add(total, r.Mul(r, big.NewRat(tokens, 1_000_000)))
	}

	add(p.InputPerMtok, u.Input)
	add(p.CachedInputPerMtok, u.CachedInput)
	add(p.CacheWritePerMtok, u.CacheWrite)
	add(p.OutputPerMtok, u.Output)

	micros := new(big.Rat).Mul(total, big.NewRat(1_000_000, 1))
	rounded := new(big.Int).Quo(micros.Num(), micros.Denom())
	if new(big.Rat).SetFrac(new(big.Int).Rem(micros.Num(), micros.Denom()), micros.Denom()).Cmp(big.NewRat(1, 2)) >= 0 {
		rounded.Add(rounded, big.NewInt(1))
	}

	v := rounded.Int64()
	return &v
}

/*
|--------------------------------------------------------------------------
| Attribute Write Gate
|--------------------------------------------------------------------------
|
| Every write of custom attributes passes here before it is stored. A key
| the workspace has defined takes only values of its type, dates as Unix
| seconds; an archived key takes none. A new key on a trusted write, from
| the team's API, a teammate or a signed token, is defined from its first
| value, up to 500 per object. A value from a browser or a customer's
| answer never defines a key. A dropped value never fails the write: it
| is counted as a problem so the team can see it.
|
*/

package attributedefs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
MaxActive is how many active definitions an object may have.
*/
const MaxActive = 500

/*
The objects that carry custom attributes.
*/
const (
	ObjectContact = "contact"
	ObjectCompany = "company"
)

/*
Write is a set of custom attribute changes about to be stored. Trusted
is set when the values come from the team's own systems rather than a
browser or a customer.
*/
type Write struct {
	WorkspaceID ids.UUID
	Object      string
	Trusted     bool
	Changes     map[string]any
	Now         time.Time
}

/*
gate is one write being checked.
*/
type gate struct {
	q      *models.Queries
	w      Write
	defs   map[string]models.AttributeDefinition
	active int
}

/*
Gate returns the changes that may be stored, each fitted to its type.
*/
func Gate(ctx context.Context, q *models.Queries, w Write) (map[string]any, error) {
	if len(w.Changes) == 0 {
		return w.Changes, nil
	}

	rows, err := q.ListAttributeDefinitions(ctx, models.ListAttributeDefinitionsParams{
		WorkspaceID: w.WorkspaceID, Object: &w.Object,
	})
	if err != nil {
		return nil, fmt.Errorf("list attribute definitions: %w", err)
	}

	g := &gate{q: q, w: w, defs: map[string]models.AttributeDefinition{}}
	for _, r := range rows {
		g.defs[r.Key] = r
		if r.Status == "active" {
			g.active++
		}
	}

	kept := map[string]any{}
	for _, key := range slices.Sorted(maps.Keys(w.Changes)) {
		value, keep, err := g.check(ctx, key, w.Changes[key])
		if err != nil {
			return nil, err
		}

		if keep {
			kept[key] = value
		}
	}

	return kept, g.touch(ctx, kept)
}

/*
check decides one key: the value to store and whether to store it. A
malformed key or value passes untouched, for the write's own checks to
refuse with an error.
*/
func (g *gate) check(ctx context.Context, key string, value any) (any, bool, error) {
	if value == nil || !attributes.ValidKey(key) || !scalar(value) {
		return value, true, nil
	}

	def, ok := g.defs[key]
	if !ok && !g.w.Trusted {
		return value, true, nil
	}

	if !ok {
		created, problem, err := g.define(ctx, key, value)
		if errors.Is(err, database.ErrNotFound) {
			return value, true, nil
		}

		if err != nil {
			return nil, false, err
		}

		if problem != "" {
			return nil, false, g.problem(ctx, key, problem, value)
		}

		def = created
	}

	if def.Status != "active" {
		return nil, false, g.problem(ctx, key, attributes.ProblemArchivedWrite, value)
	}

	fitted, problem := attributes.Coerce(def.DataType, Options(def), value)
	if problem != "" {
		return nil, false, g.problem(ctx, key, problem, value)
	}

	return fitted, true, nil
}

/*
define makes a definition for a key first seen on a trusted write. When
another write defined it a moment earlier, it reports not found and the
value is kept as sent.
*/
func (g *gate) define(ctx context.Context, key string, value any) (models.AttributeDefinition, string, error) {
	if g.active >= MaxActive {
		return models.AttributeDefinition{}, attributes.ProblemKeyLimitReached, nil
	}

	def, err := g.q.CreateAttributeDefinition(ctx, models.CreateAttributeDefinitionParams{
		ID: ids.NewV7At(g.w.Now), WorkspaceID: g.w.WorkspaceID, Object: g.w.Object, Key: key, Label: Label(key),
		DataType: attributes.Infer(key, value), Options: json.RawMessage(`[]`), Origin: "observed", Now: g.w.Now,
	})
	if err != nil {
		return def, "", fmt.Errorf("define attribute %s: %w", key, err)
	}

	g.defs[key] = def
	g.active++

	return def, "", nil
}

/*
problem counts a dropped value.
*/
func (g *gate) problem(ctx context.Context, key, code string, value any) error {
	sample := attributes.Shape(value)
	err := g.q.RecordAttributeProblem(ctx, models.RecordAttributeProblemParams{
		WorkspaceID: g.w.WorkspaceID, Object: g.w.Object, Key: key, Code: code, Sample: &sample, Now: g.w.Now,
	})
	if err != nil {
		return fmt.Errorf("record attribute problem: %w", err)
	}

	return nil
}

/*
touch records when the stored keys last took a value.
*/
func (g *gate) touch(ctx context.Context, kept map[string]any) error {
	var keys []string
	for key, value := range kept {
		if _, defined := g.defs[key]; defined && value != nil {
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		return nil
	}

	err := g.q.TouchAttributeDefinitions(ctx, models.TouchAttributeDefinitionsParams{
		Now: g.w.Now, WorkspaceID: g.w.WorkspaceID, Object: g.w.Object, Keys: keys,
		StaleBefore: g.w.Now.Add(-time.Hour),
	})
	if err != nil {
		return fmt.Errorf("touch attribute definitions: %w", err)
	}

	return nil
}

/*
Options returns a list definition's allowed values.
*/
func Options(def models.AttributeDefinition) []string {
	var out []string
	_ = json.Unmarshal(def.Options, &out)

	return out
}

/*
Label turns a key into words: trial_ends_at reads "Trial ends at".
*/
func Label(key string) string {
	words := strings.ReplaceAll(key, "_", " ")
	if words == "" {
		return key
	}

	return strings.ToUpper(words[:1]) + words[1:]
}

/*
scalar reports whether a value is text, a number or a boolean.
*/
func scalar(value any) bool {
	switch value.(type) {
	case string, float64, json.Number, bool:
		return true
	}

	return false
}

/*
|--------------------------------------------------------------------------
| Fields A Rule May Read
|--------------------------------------------------------------------------
|
| The builder offers the registered fields of a saved audience and the
| workspace's custom attributes: defined ones with their type, label and
| list values, archived ones left out, and keys in use without a
| definition typed from a value seen. A key ending in _at is read as a
| date, as the widget's own signed_up_at is.
|
*/

package audiences

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
Field is one field the builder offers: its key, its type, where it is
known, how far it is trusted, its fixed options and its operators.
Window is set on event counts, which may be narrowed to the last days.
*/
type Field struct {
	Key     string
	Type    audience.FieldType
	Where   audience.Location
	Trust   audience.Trust
	Options []string
	Ops     []string
	Custom  bool
	Label   string
	Window  bool
}

/*
Fields lists what a saved audience may read in this workspace.
*/
func (a *Audiences) Fields(ctx context.Context, m authz.Membership) ([]Field, error) {
	if !m.Can("audiences:read") {
		return nil, exceptions.ErrForbidden
	}

	catalog := audience.Base()
	surface := audience.SavedAudience()
	var out []Field

	for _, f := range catalog.Fields() {
		if !available(surface, f) {
			continue
		}

		out = append(out, Field{
			Key: f.Key, Type: f.Type, Where: f.Where, Trust: f.Trust, Options: f.Options,
			Ops: audience.OpsOf(f, ""),
		})
	}

	custom, err := a.customFields(ctx, m, catalog)

	return append(out, custom...), err
}

/*
customFields lists the custom attributes rules may read.
*/
func (a *Audiences) customFields(ctx context.Context, m authz.Membership, catalog *audience.Catalog,
) ([]Field, error) {
	var out []Field
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), m.Workspace.ID
		attrs, err := attributeFields(ctx, q, ws, catalog)
		if err != nil {
			return err
		}

		events, err := eventFieldsOf(ctx, q, ws, catalog)
		out = slices.Concat(attrs, events)

		return err
	})

	return out, err
}

/*
attributeFields lists the defined custom attributes, then the keys in
use that have no definition.
*/
func attributeFields(ctx context.Context, q *models.Queries, ws ids.UUID, catalog *audience.Catalog,
) ([]Field, error) {
	defs, err := q.ListAttributeDefinitions(ctx, models.ListAttributeDefinitionsParams{WorkspaceID: ws})
	if err != nil {
		return nil, fmt.Errorf("list attribute definitions: %w", err)
	}

	var out []Field
	defined := map[string]bool{}
	for _, d := range defs {
		defined[d.Object+"."+d.Key] = true
		if d.Status == "active" {
			out = append(out, definedField(catalog, d))
		}
	}

	keys, err := q.CustomAttributeKeys(ctx, ws)
	for _, k := range keys {
		if !defined[k.Object+"."+k.Key] {
			out = append(out, customField(catalog, k))
		}
	}

	return out, wrap("list custom attribute keys", err)
}

/*
eventFieldsOf lists the fields of the workspace's active events.
*/
func eventFieldsOf(ctx context.Context, q *models.Queries, ws ids.UUID, catalog *audience.Catalog,
) ([]Field, error) {
	names, err := q.ListEventDefinitions(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list event definitions: %w", err)
	}

	var out []Field
	for _, e := range names {
		if e.Status == "active" {
			out = append(out, eventFields(catalog, e)...)
		}
	}

	return out, nil
}

/*
definedField describes a defined custom attribute.
*/
func definedField(catalog *audience.Catalog, d models.AttributeDefinition) Field {
	key := d.Object + ".custom." + d.Key
	f, _ := catalog.Lookup(key)
	t := map[string]audience.FieldType{
		"number": audience.TypeNumber, "boolean": audience.TypeBool, "date": audience.TypeDate,
		"list": audience.TypeList,
	}[d.DataType]

	if t == "" {
		t = audience.TypeString
	}

	return Field{
		Key: key, Type: t, Where: f.Where, Trust: f.Trust, Ops: audience.OpsOf(f, t), Custom: true, Label: d.Label,
		Options: attributedefs.Options(d),
	}
}

/*
available reports whether a surface may read a field.
*/
func available(s audience.Surface, f audience.Field) bool {
	return slices.Contains(s.Where, f.Where)
}

/*
customField describes one custom attribute key seen in use.
*/
func customField(catalog *audience.Catalog, k models.CustomAttributeKeysRow) Field {
	key := k.Object + ".custom." + k.Key
	f, _ := catalog.Lookup(key)
	t := map[string]audience.FieldType{
		"number": audience.TypeNumber, "boolean": audience.TypeBool,
	}[k.Kind]

	if t == "" {
		t = audience.TypeString
		if strings.HasSuffix(k.Key, "_at") {
			t = audience.TypeDate
		}
	}

	return Field{Key: key, Type: t, Where: f.Where, Trust: f.Trust, Ops: audience.OpsOf(f, t), Custom: true}
}

/*
eventFields are the fields of one event: how many times, the first time
and the last time.
*/
func eventFields(catalog *audience.Catalog, e models.EventDefinition) []Field {
	base := "event." + e.Name
	f, _ := catalog.Lookup(base + ".count")
	field := func(suffix, words string, t audience.FieldType) Field {
		return Field{
			Key: base + suffix, Type: t, Where: f.Where, Trust: f.Trust, Ops: audience.OpsOf(f, t), Custom: true,
			Label: e.Label + ": " + words, Window: suffix == ".count",
		}
	}

	return []Field{
		field(".count", "times", audience.TypeNumber),
		field(".first_at", "first time", audience.TypeDate),
		field(".last_at", "last time", audience.TypeDate),
	}
}

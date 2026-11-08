/*
|--------------------------------------------------------------------------
| Fields A Rule May Read
|--------------------------------------------------------------------------
|
| The builder offers the registered fields of a saved audience, and the
| custom attributes the workspace actually sends, typed from a value
| seen. A key ending in _at is read as a date, as the widget's own
| signed_up_at is.
|
*/

package audiences

import (
	"context"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
Field is one field the builder offers: its key, its type, where it is
known, how far it is trusted, its fixed options and its operators.
*/
type Field struct {
	Key     string
	Type    audience.FieldType
	Where   audience.Location
	Trust   audience.Trust
	Options []string
	Ops     []string
	Custom  bool
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

	var keys []models.CustomAttributeKeysRow
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		keys, err = models.New(tx).CustomAttributeKeys(ctx, m.Workspace.ID)

		return wrap("list custom attribute keys", err)
	})

	for _, k := range keys {
		out = append(out, customField(catalog, k))
	}

	return out, err
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

/*
|--------------------------------------------------------------------------
| Attribute Definitions
|--------------------------------------------------------------------------
|
| The team names and types the custom attributes its customers carry.
| Keys already in use are listed as observed the first time anyone looks,
| so the list starts with what the workspace really sends. A type never
| changes once set; archiving stops new values while keeping old ones,
| and is refused while an audience or a workflow reads the attribute;
| only an archived attribute may be deleted, which removes its values.
| Reading takes contacts:read; changing takes attributes:manage.
|
*/

package attributes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	values "github.com/dimmihq/dimmi/app/support/attributes"
)

/*
The failures of attribute definitions.
*/
var (
	ErrNotFound = fault.New(fault.NotFound, "attribute_not_found", "No attribute of this workspace has this id.")
	ErrExists   = fault.New(fault.Conflict, "attribute_exists", "This attribute is already defined.").OnField("key")
	ErrKey      = fault.New(fault.Invalid, "key_invalid",
		"Keys start with a letter and use lowercase letters, digits and underscores, 64 at most.").OnField("key")
	ErrReserved = fault.New(fault.Invalid, "key_reserved", "This key names a standard field.").OnField("key")
	ErrType     = fault.New(fault.Invalid, "data_type_invalid", "Pick text, number, yes or no, date or list.").
			OnField("data_type")
	ErrOptions = fault.New(fault.Invalid, "options_invalid", "A list needs 1 to 200 different values.").
			OnField("options")
	ErrLabel = fault.New(fault.Invalid, "label_invalid", "Labels are 1 to 80 characters.").OnField("label")
	ErrLimit = fault.New(fault.Invalid, "attribute_limit_reached",
		"A workspace may define 500 active attributes for each of contacts and companies.")
	ErrInUse = fault.New(fault.Conflict, "attribute_in_use",
		"Audiences or workflows read this attribute. Change them first.")
	ErrActive = fault.New(fault.Conflict, "attribute_active", "Archive the attribute before deleting it.")
)

/*
Definitions manages a workspace's attribute definitions.
*/
type Definitions struct {
	DB    *database.DB
	Clock clock.Clock
	IDs   *ids.Registry
}

/*
Definition is an attribute definition and whether browsers may write it.
*/
type Definition struct {
	models.AttributeDefinition
	ClientWritable bool
}

/*
CreateInput defines a new attribute.
*/
type CreateInput struct {
	Membership  authz.Membership
	Object      string
	Key         string
	Label       string
	Description *string
	DataType    string
	Options     []string
}

/*
reserved are the standard fields a custom key may not shadow.
*/
func reserved() []string {
	return []string{
		"email", "name", "phone", "external_id", "kind", "locale", "timezone", "country", "region", "city",
		"created_at", "first_seen_at", "last_seen_at", "tags", "company", "domain", "id",
		"claimed_user_id", "test_member_id",
	}
}

/*
List returns the definitions, first recording keys in use that have
none.
*/
func (d *Definitions) List(ctx context.Context, m authz.Membership, object *string) ([]Definition, error) {
	if !m.Can("contacts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []Definition
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), m.Workspace.ID
		if err := discover(ctx, q, ws, d.Clock.Now()); err != nil {
			return err
		}

		rows, err := q.ListAttributeDefinitions(ctx, models.ListAttributeDefinitionsParams{
			WorkspaceID: ws, Object: object,
		})
		if err != nil {
			return fmt.Errorf("list attribute definitions: %w", err)
		}

		writable, err := writableKeys(ctx, q, ws)
		for _, r := range rows {
			on := r.Object == attributedefs.ObjectContact && slices.Contains(writable, r.Key)
			out = append(out, Definition{AttributeDefinition: r, ClientWritable: on})
		}

		return err
	})

	return out, err
}

/*
discover defines the keys in use that have no definition, typed from a
value seen.
*/
func discover(ctx context.Context, q *models.Queries, ws ids.UUID, now time.Time) error {
	keys, err := q.CustomAttributeKeys(ctx, ws)
	if err != nil {
		return fmt.Errorf("list custom attribute keys: %w", err)
	}

	for _, k := range keys {
		if !values.ValidKey(k.Key) {
			continue
		}

		_, err := q.CreateAttributeDefinition(ctx, models.CreateAttributeDefinitionParams{
			ID: ids.NewV7At(now), WorkspaceID: ws, Object: k.Object, Key: k.Key, Label: attributedefs.Label(k.Key),
			DataType: typeOfKind(k.Key, k.Kind), Options: json.RawMessage(`[]`), Origin: "observed", Now: now,
		})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("define attribute %s: %w", k.Key, err)
		}
	}

	return nil
}

/*
typeOfKind is the type of a key from the JSON kind of a value seen.
*/
func typeOfKind(key, kind string) string {
	switch kind {
	case "boolean":
		return values.TypeBoolean
	case "number":
		return values.Infer(key, 1.0)
	}

	return values.Infer(key, "")
}

/*
Create defines a new attribute.
*/
func (d *Definitions) Create(ctx context.Context, in CreateInput) (Definition, error) {
	m := in.Membership
	if !m.Can("attributes:manage") {
		return Definition{}, exceptions.ErrForbidden
	}

	if err := checkNew(in); err != nil {
		return Definition{}, err
	}

	options, _ := json.Marshal(nonNil(in.Options))
	var out Definition
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), m.Workspace.ID, d.Clock.Now()
		active, err := q.CountActiveAttributeDefinitions(ctx, models.CountActiveAttributeDefinitionsParams{
			WorkspaceID: ws, Object: in.Object,
		})
		if err != nil {
			return fmt.Errorf("count attribute definitions: %w", err)
		}

		if active >= attributedefs.MaxActive {
			return ErrLimit
		}

		row, err := q.CreateAttributeDefinition(ctx, models.CreateAttributeDefinitionParams{
			ID: ids.NewV7At(now), WorkspaceID: ws, Object: in.Object, Key: in.Key, Label: strings.TrimSpace(in.Label),
			Description: in.Description, DataType: in.DataType, Options: options, Origin: "created", Now: now,
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrExists
		}

		out = Definition{AttributeDefinition: row}

		return wrap("define attribute", err)
	})

	return out, err
}

/*
checkNew checks a new definition's key, label, type and options.
*/
func checkNew(in CreateInput) error {
	switch {
	case in.Object != attributedefs.ObjectContact && in.Object != attributedefs.ObjectCompany:
		return fault.New(fault.Invalid, "object_invalid", "Pick contact or company.").OnField("object")
	case !values.ValidKey(in.Key):
		return ErrKey
	case slices.Contains(reserved(), in.Key):
		return ErrReserved
	case !slices.Contains(values.Types(), in.DataType):
		return ErrType
	}

	if n := utf8.RuneCountInString(strings.TrimSpace(in.Label)); n < 1 || n > 80 {
		return ErrLabel
	}

	return checkOptions(in.DataType, in.Options)
}

/*
checkOptions refuses a list without values, and values on other types.
*/
func checkOptions(dataType string, options []string) error {
	if dataType != values.TypeList {
		if len(options) > 0 {
			return ErrOptions
		}

		return nil
	}

	seen := map[string]bool{}
	for _, o := range options {
		if strings.TrimSpace(o) == "" || seen[o] {
			return ErrOptions
		}

		seen[o] = true
	}

	if len(options) < 1 || len(options) > 200 {
		return ErrOptions
	}

	return nil
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}

/*
find returns one definition of the workspace.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.AttributeDefinition, error) {
	row, err := q.FindAttributeDefinition(ctx, models.FindAttributeDefinitionParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return row, ErrNotFound
	}

	return row, wrap("find attribute definition", err)
}

/*
wrap names what failed, leaving no error alone.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}

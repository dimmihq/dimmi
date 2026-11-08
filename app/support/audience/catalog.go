/*
|--------------------------------------------------------------------------
| Field Catalog
|--------------------------------------------------------------------------
|
| Every field a rule may read is registered once: its type, where its
| value is known, and how far it can be trusted. A feature that brings
| its own fields, such as workflows with conversation and message
| fields, extends the base catalog with its registrations instead of
| inventing a second language.
|
| Where says who can evaluate the field: the server for contact and
| company data, the browser for the page being viewed, or a conversation
| for fields that only exist once one does. Trust says whether a value
| came from the customer's own systems (verified), from what the
| server saw (observed), or from whatever the browser claimed.
|
*/

package audience

import (
	"maps"
	"slices"
	"strings"
)

/*
FieldType is the kind of value a field holds, which decides the
operators that apply.
*/
type FieldType string

/*
The field types. Auto takes the type of whatever value is there, as for
custom attributes and run variables without a declared type.
*/
const (
	TypeString FieldType = "string"
	TypeNumber FieldType = "number"
	TypeBool   FieldType = "bool"
	TypeDate   FieldType = "date"
	TypeList   FieldType = "list"
	TypeSet    FieldType = "set"
	TypeURL    FieldType = "url"
	TypeID     FieldType = "id"
	TypeAuto   FieldType = "auto"
	TypeSaved  FieldType = "saved_audience"
)

/*
Location says where a field's value is known.
*/
type Location string

/*
The locations a value can be known at.
*/
const (
	Server       Location = "server"
	Client       Location = "client"
	Conversation Location = "conversation"
)

/*
Trust says how far a field's value can be believed.
*/
type Trust string

/*
The trust levels.
*/
const (
	Verified Trust = "verified"
	Observed Trust = "observed"
	Claimed  Trust = "claimed"
)

/*
Field registers one field. Key is the full name, or a prefix ending in
a dot when Prefix is set, as "contact.custom." for every custom
attribute. Options lists the allowed values of a list field. Ops, when
set, narrows the operators of the type.
*/
type Field struct {
	Key      string
	Type     FieldType
	Where    Location
	Trust    Trust
	Prefix   bool
	Options  []string
	Ops      []string
	Label    string
	Triggers []string
}

/*
Catalog is an immutable set of registered fields.
*/
type Catalog struct {
	exact    map[string]Field
	prefixes []Field
}

/*
NewCatalog registers the fields. A later field replaces an earlier one
with the same key.
*/
func NewCatalog(fields ...Field) *Catalog {
	c := &Catalog{exact: map[string]Field{}}
	return c.With(fields...)
}

/*
With returns a new catalog holding this one's fields and the given ones.
*/
func (c *Catalog) With(fields ...Field) *Catalog {
	out := &Catalog{exact: maps.Clone(c.exact), prefixes: append([]Field(nil), c.prefixes...)}
	for _, f := range fields {
		if f.Prefix {
			out.prefixes = append(out.prefixes, f)
			continue
		}

		out.exact[f.Key] = f
	}

	return out
}

/*
Lookup finds the field a key names: an exact registration first, then
the longest registered prefix.
*/
func (c *Catalog) Lookup(key string) (Field, bool) {
	if f, ok := c.exact[key]; ok {
		return f, true
	}

	var best Field
	found := false

	for _, f := range c.prefixes {
		if strings.HasPrefix(key, f.Key) && len(key) > len(f.Key) && len(f.Key) > len(best.Key) {
			best, found = f, true
		}
	}

	return best, found
}

/*
Fields returns the fields registered by exact key, in key order, for
builders that list what a rule may read.
*/
func (c *Catalog) Fields() []Field {
	out := make([]Field, 0, len(c.exact))
	for _, key := range slices.Sorted(maps.Keys(c.exact)) {
		out = append(out, c.exact[key])
	}

	return out
}

/*
OpsOf returns the operators a field takes. A field of unknown type
takes the operators of the given type when one is known.
*/
func OpsOf(f Field, known FieldType) []string {
	if f.Type == TypeAuto && known != "" {
		return opsFor(known)
	}

	return allowedOps(f)
}

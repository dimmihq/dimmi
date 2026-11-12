/*
|--------------------------------------------------------------------------
| Intercom
|--------------------------------------------------------------------------
|
| Intercom's ticket types carry their attributes and the states they
| use; each state has a category, which becomes its group, an internal
| label the team reads and an external label customers read. Customer,
| back-office and tracker types keep their kind. The default title and
| description attributes become the ticket's own title and description;
| the others become fields keyed by their name.
|
*/

package ticketimports

import (
	"context"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
IntercomAPI is Intercom's API address.
*/
const IntercomAPI = "https://api.intercom.io"

/*
Intercom reads tickets from an Intercom workspace with an access token.
BaseURL is IntercomAPI unless a test or a region says otherwise.
*/
type Intercom struct {
	HTTP    *httpclient.Client
	BaseURL string
	Token   string

	people map[string]person
	admins map[string]string
}

/*
person is a contact's email and name.
*/
type person struct {
	email, name string
}

/*
intercomState is a ticket state as Intercom shows it.
*/
type intercomState struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Internal string `json:"internal_label"`
	External string `json:"external_label"`
}

/*
intercomAttribute is one attribute of a ticket type.
*/
type intercomAttribute struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	DataType string `json:"data_type"`
	Default  bool   `json:"default"`
	Visible  bool   `json:"visible_to_contacts"`
	Archived bool   `json:"archived"`
	Input    struct {
		Multiline bool `json:"multiline"`
		Options   []struct {
			Label    string `json:"label"`
			Archived bool   `json:"archived"`
		} `json:"list_options"`
	} `json:"input_options"`
}

/*
intercomType is a ticket type with its attributes and states.
*/
type intercomType struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
	Attributes  struct {
		Data []intercomAttribute `json:"data"`
	} `json:"ticket_type_attributes"`
	States struct {
		Data []intercomState `json:"data"`
	} `json:"ticket_states"`
}

/*
Name returns intercom.
*/
func (s *Intercom) Name() string { return "intercom" }

/*
reader returns the authenticated reader.
*/
func (s *Intercom) reader() reader {
	return reader{http: s.HTTP, headers: map[string]string{
		"Authorization": "Bearer " + s.Token, "Intercom-Version": "2.11",
	}}
}

/*
url returns the API address of the path.
*/
func (s *Intercom) url(path string) string {
	base := s.BaseURL
	if base == "" {
		base = IntercomAPI
	}

	return strings.TrimRight(base, "/") + path
}

/*
Catalog reads the ticket states and types.
*/
func (s *Intercom) Catalog(ctx context.Context) (Catalog, error) {
	var states struct {
		Data []intercomState `json:"data"`
	}
	if err := s.reader().get(ctx, s.url("/ticket_states"), &states); err != nil {
		return Catalog{}, err
	}

	var types struct {
		Data []intercomType `json:"data"`
	}
	if err := s.reader().get(ctx, s.url("/ticket_types"), &types); err != nil {
		return Catalog{}, err
	}

	var out Catalog
	for _, st := range states.Data {
		out.States = append(out.States, State{
			ExternalID: st.ID, Group: intercomGroup(st.Category), Name: st.Internal, CustomerLabel: st.External,
		})
	}

	for _, t := range types.Data {
		if !t.Archived {
			out.Types = append(out.Types, intercomTypeOf(t))
		}
	}

	return out, nil
}

/*
intercomGroup maps a state category to its group.
*/
func intercomGroup(category string) string {
	switch category {
	case "in_progress":
		return "in_progress"
	case "waiting_on_customer":
		return "waiting_customer"
	case "resolved":
		return "resolved"
	}

	return "submitted"
}

/*
intercomKind maps a type's category to its kind.
*/
func intercomKind(category string) string {
	switch strings.ToLower(category) {
	case "back-office", "back_office":
		return "back_office"
	case "tracker":
		return "tracker"
	}

	return "customer"
}

/*
intercomTypeOf reads a type with its fields and states.
*/
func intercomTypeOf(t intercomType) Type {
	out := Type{ExternalID: t.ID, Kind: intercomKind(t.Category), Name: t.Name, Description: t.Description}

	for _, a := range t.Attributes.Data {
		if a.Default || a.Archived {
			continue
		}

		out.Fields = append(out.Fields, intercomField(a))
	}

	for _, st := range t.States.Data {
		out.States = append(out.States, st.ID)
	}

	return out
}

/*
intercomField reads an attribute as a field.
*/
func intercomField(a intercomAttribute) Field {
	f := Field{ExternalID: a.ID, Key: KeyOf(a.Name), Label: a.Name, Customer: a.Visible}

	switch a.DataType {
	case "list":
		f.DataType = "single_choice"
		for _, o := range a.Input.Options {
			if !o.Archived {
				f.Options = append(f.Options, o.Label)
			}
		}
	case "integer":
		f.DataType = "number"
	case "decimal":
		f.DataType = "decimal"
	case "boolean":
		f.DataType = "boolean"
	case "datetime":
		f.DataType = "date"
	default:
		f.DataType = "short_text"
		if a.Input.Multiline {
			f.DataType = "long_text"
		}
	}

	return f
}

/*
nonKey is what a field key may not hold.
*/
var nonKey = regexp.MustCompile(`[^a-z0-9]+`)

/*
KeyOf turns a label into a field key: lowercase letters, digits and
underscores, starting with a letter, at most 40.
*/
func KeyOf(label string) string {
	key := strings.Trim(nonKey.ReplaceAllString(strings.ToLower(label), "_"), "_")
	if key == "" || key[0] < 'a' || key[0] > 'z' {
		key = "f_" + key
	}

	if len(key) > 40 {
		key = strings.TrimRight(key[:40], "_")
	}

	return key
}

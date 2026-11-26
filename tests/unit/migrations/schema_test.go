package migrations_test

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/migrations"
)

/*
schemaFile is the published JSON Schema of the import format.
*/
const schemaFile = "../../../docs/public/migrations/import-format-v1.schema.json"

/*
schema is the part of the JSON Schema the test reads.
*/
type schema struct {
	Properties struct {
		Kind struct {
			Enum []string `json:"enum"`
		} `json:"kind"`
	} `json:"properties"`
	Defs map[string]struct {
		Properties map[string]json.RawMessage `json:"properties"`
	} `json:"$defs"`
}

/*
fields returns the JSON names of a struct's fields, its embedded
structs' included.
*/
func fields(t reflect.Type) []string {
	var out []string

	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous {
			out = append(out, fields(f.Type)...)

			continue
		}

		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}

	sort.Strings(out)

	return out
}

func TestImportFormatSchema(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(schemaFile)
	require.NoError(t, err)

	var s schema
	require.NoError(t, json.Unmarshal(raw, &s))

	t.Run("lists every kind the server reads", func(t *testing.T) {
		t.Parallel()
		assert.ElementsMatch(t, migrations.Kinds(), s.Properties.Kind.Enum)
	})

	t.Run("describes every field of every record, and no other", func(t *testing.T) {
		t.Parallel()

		types := map[string]any{
			migrations.KindTeammate: migrations.Teammate{}, migrations.KindTeam: migrations.Team{},
			migrations.KindTag: migrations.Tag{}, migrations.KindAttributeDefinition: migrations.AttributeDefinition{},
			migrations.KindCompany: migrations.Company{}, migrations.KindContact: migrations.Contact{},
			migrations.KindContactNote: migrations.ContactNote{}, migrations.KindTicketType: migrations.TicketType{},
			migrations.KindTicket: migrations.Ticket{}, migrations.KindHelpCenter: migrations.HelpCenter{},
			migrations.KindCollection: migrations.Collection{}, migrations.KindArticle: migrations.Article{},
			migrations.KindSnippet: migrations.Snippet{}, migrations.KindSavedReply: migrations.SavedReply{},
			migrations.KindSchedule: migrations.Schedule{}, migrations.KindConversation: migrations.Conversation{},
			"entry": migrations.Entry{}, "event": migrations.Event{}, "author": migrations.Author{},
			"translation": migrations.Translation{},
		}

		for name, v := range types {
			def, ok := s.Defs[name]
			require.True(t, ok, "no $defs entry for %s", name)

			var described []string
			for key := range def.Properties {
				described = append(described, key)
			}

			slices.Sort(described)
			assert.Equal(t, fields(reflect.TypeOf(v)), described, "fields of %s", name)
		}
	})
}

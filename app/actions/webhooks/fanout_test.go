package webhooks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

func TestNoteEventsReachOnlyEndpointsThatIncludeNotes(t *testing.T) {
	t.Parallel()

	withNotes := models.WebhookEndpoint{IncludeNotes: true}
	withoutNotes := models.WebhookEndpoint{}

	t.Run("a ticket note is a note to every endpoint", func(t *testing.T) {
		t.Parallel()

		snap := Snapshot{Note: aboutNote("ticket.note_created")}

		assert.True(t, receives(withNotes, snap))
		assert.False(t, receives(withoutNotes, snap))
	})

	t.Run("every notes opt-in event outside messages carries a note", func(t *testing.T) {
		t.Parallel()

		for _, entry := range appevents.WebhookCatalog() {
			if entry.Object == subjectMessage {
				continue
			}

			assert.Equal(t, entry.NotesOptIn, aboutNote(entry.Type), entry.Type)
		}
	})

	t.Run("a message event leaves it to the snapshot", func(t *testing.T) {
		t.Parallel()

		assert.False(t, aboutNote("message.created"))
		assert.True(t, receives(withoutNotes, Snapshot{Note: aboutNote("message.created")}))
	})

	t.Run("other events and unknown types are not notes", func(t *testing.T) {
		t.Parallel()

		assert.False(t, aboutNote("ticket.created"))
		assert.False(t, aboutNote("nothing.happened"))
	})
}

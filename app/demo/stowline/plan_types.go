/*
|--------------------------------------------------------------------------
| What A Plan Holds
|--------------------------------------------------------------------------
|
| The rows a plan describes before they are written: companies, the
| people who write in, and conversations with their messages.
|
*/

package stowline

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Company is one customer company.
*/
type Company struct {
	ID                                    ids.UUID
	Name, Domain, Plan, Industry, Country string
	Platform                              string
	Seats, MonthlyLabels                  int
	CreatedAt                             time.Time
}

/*
Contact is one person who writes in, at a company or on their own.
*/
type Contact struct {
	ID                             ids.UUID
	Company                        int
	Name, Email, Phone, ExternalID string
	Locale, Timezone, Country      string
	City, Role, Plan               string
	Lead                           bool
	FirstSeen, LastSeen            time.Time
}

/*
Message is one message of a planned conversation. Author is contact,
member, agent or note.
*/
type Message struct {
	Author string
	Body   string
	At     time.Time
}

/*
Conversation is one planned conversation and how it went.
*/
type Conversation struct {
	ID            ids.UUID
	Contact       int
	Topic         string
	Inbox         string
	Channel       string
	Subject       string
	Status        string
	Priority      int
	Teammate      int
	Team          string
	Tags          []string
	Messages      []Message
	CreatedAt     time.Time
	ClosedAt      time.Time
	SnoozedUntil  time.Time
	AI            string
	HandoffReason string
	Rating        int
	RatingComment string
	Language      string
	Article       string
	// Planted marks a hand-written case, which nothing later in the seed may change.
	Planted bool
}

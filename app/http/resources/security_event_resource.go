/*
|--------------------------------------------------------------------------
| Security Event Resource
|--------------------------------------------------------------------------
|
| One entry of a person's own security log. The device id that tells a
| sign-in from a new device stays on the server: it is derived from the
| browser's device cookie and means nothing to the person.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
hiddenMetadata lists the security log details never shown.
*/
const hiddenMetadata = "device_id"

/*
SecurityEvent is one entry of the security log.
*/
type SecurityEvent struct {
	ID          string         `json:"id"`
	Object      string         `json:"object"`
	Type        string         `json:"type"`
	IP          *string        `json:"ip"`
	UserAgent   *string        `json:"user_agent"`
	DeviceLabel *string        `json:"device_label"`
	Country     *string        `json:"country"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

/*
NewSecurityEvents shapes a page of the security log, in the order
given.
*/
func NewSecurityEvents(events []models.SecurityEvent) []SecurityEvent {
	out := make([]SecurityEvent, 0, len(events))

	for _, e := range events {
		metadata := map[string]any{}
		if err := json.Unmarshal(e.Metadata, &metadata); err != nil || metadata == nil {
			metadata = map[string]any{}
		}

		delete(metadata, hiddenMetadata)

		event := SecurityEvent{
			ID:          models.PublicID(models.PrefixSecurityEvent, e.ID),
			Object:      "security_event",
			Type:        e.Type,
			UserAgent:   e.UserAgent,
			DeviceLabel: e.DeviceLabel,
			Country:     e.Country,
			Metadata:    metadata,
			CreatedAt:   e.CreatedAt.UTC(),
		}

		if e.Ip != nil {
			ip := e.Ip.String()
			event.IP = &ip
		}

		out = append(out, event)
	}

	return out
}

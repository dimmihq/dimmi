/*
|--------------------------------------------------------------------------
| Retention Settings
|--------------------------------------------------------------------------
|
| How long a workspace keeps conversations and the files around them. A
| workspace that never chose keeps closed conversations for ever, spam
| and the trash for 30 days, raw email for 30 days after it is read, and
| data exports for 7 days. ChangedAt records the last change, so a
| shorter period never takes effect the day it is set.
|
*/

package models

import (
	"encoding/json"
	"time"
)

/*
Retention is a workspace's retention periods, in days.
ClosedConversationDays is nil when closed conversations are kept for
ever.
*/
type Retention struct {
	ClosedConversationDays *int       `json:"closed_conversation_days"`
	SpamDays               int        `json:"spam_days"`
	TrashDays              int        `json:"trash_days"`
	RawEmailDays           int        `json:"raw_email_days"`
	ExportDays             int        `json:"export_days"`
	ChangedAt              *time.Time `json:"changed_at,omitempty"`
}

/*
DefaultRetention is what a workspace keeps until it chooses otherwise.
*/
func DefaultRetention() Retention {
	return Retention{SpamDays: 30, TrashDays: 30, RawEmailDays: 30, ExportDays: 7}
}

/*
Retention returns the workspace's retention periods over the defaults.
*/
func (w Workspace) Retention() Retention {
	var settings struct {
		Retention Retention `json:"retention"`
	}

	settings.Retention = DefaultRetention()

	if len(w.Settings) > 0 && json.Unmarshal(w.Settings, &settings) != nil {
		return DefaultRetention()
	}

	return settings.Retention
}

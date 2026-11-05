/*
|--------------------------------------------------------------------------
| Workspace Behavior
|--------------------------------------------------------------------------
|
| Settings of a workspace, read from and written to its settings
| document in one place, so the policy that enforces a setting and the
| endpoint that changes it always agree on where it lives. Settings nest
| by area, such as {"security": {"require_two_factor": true}}, and every
| key a change does not touch is kept as it was.
|
*/

package models

import (
	"encoding/json"
	"fmt"
)

/*
RequiresTwoFactor reports whether every member must protect their
account with a second factor to work in the workspace. Settings that do
not read as a document require nothing.
*/
func (w Workspace) RequiresTwoFactor() bool {
	return w.Security().RequireTwoFactor
}

/*
WithTwoFactorRequired returns the settings document with the
requirement of a second factor set to on, keeping every other key.
*/
func WithTwoFactorRequired(settings json.RawMessage, on bool) (json.RawMessage, error) {
	return withSecurityKeys(settings, map[string]any{"require_two_factor": on})
}

/*
ShowsViewersOnList reports whether the inbox list shows who views each
conversation. It does unless the workspace turned it off.
*/
func (w Workspace) ShowsViewersOnList() bool {
	var settings struct {
		Inbox struct {
			ShowViewersOnList *bool `json:"show_viewers_on_list"`
		} `json:"inbox"`
	}

	if len(w.Settings) == 0 || json.Unmarshal(w.Settings, &settings) != nil {
		return true
	}

	return settings.Inbox.ShowViewersOnList == nil || *settings.Inbox.ShowViewersOnList
}

/*
SeenReceiptShowsName reports whether the customer's widget names the
teammate who read their message. It does not unless the workspace
turned it on: by default the customer only learns that the team saw it.
*/
func (w Workspace) SeenReceiptShowsName() bool {
	var settings struct {
		Inbox struct {
			SeenReceiptShowsName bool `json:"seen_receipt_shows_name"`
		} `json:"inbox"`
	}

	if len(w.Settings) == 0 || json.Unmarshal(w.Settings, &settings) != nil {
		return false
	}

	return settings.Inbox.SeenReceiptShowsName
}

/*
WithSetting returns the settings document with the key of the area set
to the value, keeping every other key.
*/
func WithSetting(settings json.RawMessage, area, key string, value any) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &values); err != nil {
			return nil, fmt.Errorf("read workspace settings: %w", err)
		}
	}

	section := map[string]json.RawMessage{}
	if raw, ok := values[area]; ok {
		if err := json.Unmarshal(raw, &section); err != nil {
			return nil, fmt.Errorf("read workspace %s settings: %w", area, err)
		}
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("write workspace setting %s: %w", key, err)
	}

	section[key] = encoded

	raw, err := json.Marshal(section)
	if err != nil {
		return nil, fmt.Errorf("write workspace %s settings: %w", area, err)
	}

	values[area] = raw

	return json.Marshal(values)
}

/*
CustomerFacingName returns the name customers see in messages and mail
from the workspace: the one set for them, or else the workspace's own
name.
*/
func (w Workspace) CustomerFacingName() string {
	if name := w.CustomerFacingNameSetting(); name != nil {
		return *name
	}

	return w.Name
}

/*
CustomerFacingNameSetting returns the name set for customers, nil when
none is, so customers see the workspace's own name.
*/
func (w Workspace) CustomerFacingNameSetting() *string {
	var settings struct {
		CustomerFacingName *string `json:"customer_facing_name"`
	}

	if len(w.Settings) == 0 || json.Unmarshal(w.Settings, &settings) != nil {
		return nil
	}

	if settings.CustomerFacingName == nil || *settings.CustomerFacingName == "" {
		return nil
	}

	return settings.CustomerFacingName
}

/*
WithCustomerFacingName returns the settings document with the name
customers see set, or removed when name is nil, keeping every other key.
*/
func WithCustomerFacingName(settings json.RawMessage, name *string) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &values); err != nil {
			return nil, fmt.Errorf("read workspace settings: %w", err)
		}
	}

	if name == nil {
		delete(values, "customer_facing_name")

		return json.Marshal(values)
	}

	raw, err := json.Marshal(*name)
	if err != nil {
		return nil, fmt.Errorf("write customer facing name: %w", err)
	}

	values["customer_facing_name"] = raw

	return json.Marshal(values)
}

/*
SnoozeWakeDefaults returns whether each teammate move wakes a snoozed
conversation when the workspace has not chosen: a reply does, a note by
someone other than the holder and handing it on do not.
*/
func SnoozeWakeDefaults() map[string]bool {
	return map[string]bool{"member_reply": true, "others_note": false, "assigned": false}
}

/*
WakesSnoozeOn reports whether the move, named by its wake reason, wakes
a snoozed conversation in the workspace. Settings live under snooze as
wake_on_reply, wake_on_others_note and wake_on_assignment.
*/
func (w Workspace) WakesSnoozeOn(reason string) bool {
	var settings struct {
		Snooze map[string]bool `json:"snooze"`
	}

	keys := map[string]string{
		"member_reply": "wake_on_reply", "others_note": "wake_on_others_note", "assigned": "wake_on_assignment",
	}

	if len(w.Settings) > 0 && json.Unmarshal(w.Settings, &settings) == nil {
		if on, ok := settings.Snooze[keys[reason]]; ok {
			return on
		}
	}

	return SnoozeWakeDefaults()[reason]
}

/*
DefaultUndoSendSeconds is how long a reply waits before it leaves, so its
writer can undo it, when the workspace sets nothing.
*/
const DefaultUndoSendSeconds = 5

/*
UndoSendSeconds returns how many seconds the workspace's replies wait
before they leave on a channel that cannot take them back.
*/
func (w Workspace) UndoSendSeconds() int {
	var settings struct {
		Inbox struct {
			UndoSendSeconds *int `json:"undo_send_seconds"`
		} `json:"inbox"`
	}

	if len(w.Settings) == 0 || json.Unmarshal(w.Settings, &settings) != nil || settings.Inbox.UndoSendSeconds == nil {
		return DefaultUndoSendSeconds
	}

	return *settings.Inbox.UndoSendSeconds
}

/*
LogoURL returns the address of the workspace's logo on this server, or
nil when it has none.
*/
func (w Workspace) LogoURL() *string {
	if w.LogoPath == nil {
		return nil
	}

	url := "/" + *w.LogoPath

	return &url
}

/*
|--------------------------------------------------------------------------
| Update Profile
|--------------------------------------------------------------------------
|
| A person's name, language and time zone are theirs to change at any
| time, with no proof beyond being signed in. Only a new name concerns
| anyone else, so only that tells their teammates. A cleared language or
| time zone means following the workspace again.
|
*/

package users

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateProfileInput is what changes. A nil Name stays as it is. A nil
Locale or Timezone stays unless its Clear flag empties it. Preferences
change only the keys sent, null returning a key to its default.
*/
type UpdateProfileInput struct {
	UserID        ids.UUID
	Name          *string
	Locale        *string
	ClearLocale   bool
	Timezone      *string
	ClearTimezone bool
	Preferences   map[string]json.RawMessage
}

/*
UpdateProfile changes the caller's own name, language, time zone and
preferences.

Events: [appevents.UserUpdated] when the name changed.

Errors: [appauth.ErrUnknownPreference], [appauth.ErrInvalidPreference],
or a wrapped database error.
*/
type UpdateProfile struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
}

/*
Handle applies the change and returns the user as they now are.
*/
func (a *UpdateProfile) Handle(ctx context.Context, in UpdateProfileInput) (models.User, error) {
	var updated models.User

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		user, err := q.LockUser(ctx, in.UserID)
		if err != nil {
			return fmt.Errorf("lock user to update: %w", err)
		}

		change, err := profileChange(user, in, a.Clock)
		if err != nil {
			return err
		}

		updated, err = q.UpdateUserProfile(ctx, change)
		if err != nil {
			return fmt.Errorf("update profile: %w", err)
		}

		if updated.Name == user.Name {
			return nil
		}

		memberships, err := membershipsOf(ctx, q, user.ID)
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.UserUpdated{
			UserID: user.ID, Changes: []string{"name"}, Memberships: memberships,
		})
	})

	return updated, err
}

/*
profileChange works out the values to store from the user as they are
and what was asked.
*/
func profileChange(user models.User, in UpdateProfileInput, c clock.Clock) (models.UpdateUserProfileParams, error) {
	out := models.UpdateUserProfileParams{
		ID: user.ID, Name: user.Name, Locale: user.Locale, Timezone: user.Timezone, Preferences: user.Preferences,
		Now: c.Now(),
	}

	if in.Name != nil {
		out.Name = str.Clean(*in.Name)
	}

	if in.Locale != nil || in.ClearLocale {
		out.Locale = in.Locale
	}

	if in.Timezone != nil || in.ClearTimezone {
		out.Timezone = in.Timezone
	}

	if len(in.Preferences) == 0 {
		return out, nil
	}

	merged, err := appauth.MergePreferences(user.Preferences, in.Preferences)
	if err != nil {
		return models.UpdateUserProfileParams{}, fmt.Errorf("change preferences: %w", err)
	}

	out.Preferences = merged

	return out, nil
}

/*
|--------------------------------------------------------------------------
| Event Names
|--------------------------------------------------------------------------
|
| The names a workspace's events go by, each with how many arrived and
| when the last did, and what one person did. A name's label and
| description change freely; archiving stops new events of it while the
| summaries already kept still answer rules. Reading takes
| contacts:read; changing takes attributes:manage, the same permission
| that shapes the rest of a workspace's customer data.
|
*/

package events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The failures of event names.
*/
var (
	ErrNotFound = fault.New(fault.NotFound, "event_not_found", "No event of this workspace has this id.")
	ErrLabel    = fault.New(fault.Invalid, "label_invalid", "Labels are 1 to 80 characters.").OnField("label")
	ErrStatus   = fault.New(fault.Invalid, "status_invalid", "Pick active or archived.").OnField("status")
)

/*
UpdateInput changes an event name. Nil fields stay as they are.
*/
type UpdateInput struct {
	Membership  authz.Membership
	ID          ids.UUID
	Label       *string
	Description *string
	Status      *string
}

/*
Summary is one person's summary of one event, with the event's name.
*/
type Summary = models.ListContactEventSummariesRow

/*
List returns the workspace's event names.
*/
func (e *Events) List(ctx context.Context, m authz.Membership) ([]models.EventDefinition, error) {
	if !m.Can("contacts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.EventDefinition
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListEventDefinitions(ctx, m.Workspace.ID)

		return wrap("list event definitions", err)
	})

	return out, err
}

/*
Update changes an event name's words or status.
*/
func (e *Events) Update(ctx context.Context, in UpdateInput) (models.EventDefinition, error) {
	m := in.Membership
	if !m.Can("attributes:manage") {
		return models.EventDefinition{}, exceptions.ErrForbidden
	}

	var out models.EventDefinition
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := q.FindEventDefinition(ctx, models.FindEventDefinitionParams{WorkspaceID: m.Workspace.ID, ID: in.ID})
		if errors.Is(err, database.ErrNotFound) {
			return ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find event definition: %w", err)
		}

		p, err := changed(row, in)
		if err != nil {
			return err
		}

		p.Now = e.Clock.Now()
		out, err = q.UpdateEventDefinition(ctx, p)

		return wrap("update event definition", err)
	})

	return out, err
}

/*
changed applies the input over the stored name.
*/
func changed(row models.EventDefinition, in UpdateInput) (models.UpdateEventDefinitionParams, error) {
	p := models.UpdateEventDefinitionParams{
		Label: row.Label, Description: row.Description, Status: row.Status, WorkspaceID: row.WorkspaceID, ID: row.ID,
	}

	if in.Label != nil {
		p.Label = strings.TrimSpace(*in.Label)
		if n := utf8.RuneCountInString(p.Label); n < 1 || n > 80 {
			return p, ErrLabel
		}
	}

	if in.Description != nil {
		p.Description = in.Description
	}

	if in.Status != nil {
		if *in.Status != "active" && *in.Status != "archived" {
			return p, ErrStatus
		}

		p.Status = *in.Status
	}

	return p, nil
}

/*
Summaries returns what one person did, newest first.
*/
func (e *Events) Summaries(ctx context.Context, m authz.Membership, contact ids.UUID) ([]Summary, error) {
	if !m.Can("contacts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []Summary
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListContactEventSummaries(ctx, models.ListContactEventSummariesParams{
			WorkspaceID: m.Workspace.ID, ContactIds: []ids.UUID{contact},
		})

		return wrap("list event summaries", err)
	})

	return out, err
}

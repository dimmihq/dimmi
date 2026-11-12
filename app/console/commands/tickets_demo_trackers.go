//go:build dev

/*
|--------------------------------------------------------------------------
| Tickets Demo Trackers And Incidents
|--------------------------------------------------------------------------
|
| Three trackers each gather dozens of conversations reporting one
| problem, linked as the reports came in over the weeks, from customers
| of paying companies so the revenue at stake adds up. An incident is
| still running, with its public log moving from investigating to
| monitoring and the conversations it caused linked; another was
| resolved weeks ago. Two trackers then message everyone linked, which
| lands for most, is held for customers who wrote since, and fails for
| any the channel cannot reach.
|
*/

package commands

import (
	"context"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
demoTracker is one tracker or incident the demo makes: what it is,
how long ago it opened, how many conversations report it, the state it
is in, and for an incident its public log.
*/
type demoTracker struct {
	key, kind, title, description, state string
	opened                               time.Duration
	reports                              int
	fields                               map[string]any
	log                                  []demoPost
}

/*
demoPost is one post to an incident's public log, some time after it
opened, moving it to a state.
*/
type demoPost struct {
	after       time.Duration
	state, body string
}

/*
demoTrackers lists the demo's trackers.
*/
func demoTrackers() []demoTracker {
	day := 24 * time.Hour

	return []demoTracker{
		{
			key: "exports", kind: "Tracker", title: "CSV exports time out for large workspaces", opened: 24 * day,
			reports: 64, state: "Being worked on",
			description: "Exports over 50,000 rows stop after 60 seconds with a blank file.",
		},
		{
			key: "ios-login", kind: "Tracker", title: "iOS app crashes right after sign-in", opened: 16 * day,
			reports: 57, state: "Investigating", description: "Crash on launch after the 6.2 update, iOS 18 only.",
		},
		{
			key: "vat", kind: "Tracker", title: "Invoices show the wrong VAT rate for EU customers", opened: 30 * day,
			reports: 52, state: "Investigating", description: "VAT is charged at 20% for every EU country.",
		},
	}
}

/*
trackers makes the trackers and links their reports.
*/
func (d *ticketDemo) trackers(ctx context.Context) error {
	for _, spec := range demoTrackers() {
		if err := d.tracker(ctx, spec); err != nil {
			return err
		}
	}

	return nil
}

/*
tracker makes one tracker or incident unless it exists, links the
conversations reporting it as they came in and posts its log.
*/
func (d *ticketDemo) tracker(ctx context.Context, spec demoTracker) error {
	now := d.app.Clock.Now()
	opened := now.Add(-spec.opened)
	external := "demo:" + spec.kind + ":" + spec.key
	by := d.mate("tracker", spec.key)

	var made tickets.Created

	err := d.write(ctx, opened, by, func(ctx context.Context, w *tickets.Writer) error {
		var err error
		made, err = w.Create(ctx, d.owner.Workspace.ID, tickets.CreateInput{
			TypeID: d.types[spec.kind], Title: spec.title, Description: spec.description, Fields: spec.fields,
			ExternalID: &external, AssigneeMember: &by.id, Priority: "high",
		})

		return err
	})
	if err != nil || !made.New {
		return err
	}

	if err := d.linkReports(ctx, made.Ticket, spec, opened); err != nil {
		return err
	}

	if spec.kind == "Incident" {
		return d.postLog(ctx, made.Ticket, spec, opened)
	}

	return d.moveTo(ctx, made.Ticket, spec.state, opened.Add(time.Hour))
}

/*
linkReports links conversations started since the tracker opened,
each when it came in.
*/
func (d *ticketDemo) linkReports(ctx context.Context, t models.Ticket, spec demoTracker, opened time.Time) error {
	convs, err := d.recentConversations(ctx, spec.reports, opened.Add(-time.Hour))
	if err != nil {
		return err
	}

	for i, c := range convs {
		at := c.at.Add(time.Duration(10+roll(120, spec.key, i)) * time.Minute)
		if at.Before(opened) {
			at = opened.Add(time.Duration(i) * time.Minute)
		}

		if latest := d.app.Clock.Now().Add(-time.Minute); at.After(latest) {
			at = latest
		}

		conv := c.id

		err := d.write(ctx, at, d.mate("link", spec.key, i), func(ctx context.Context, w *tickets.Writer) error {
			_, err := w.Link(ctx, t, tickets.LinkInput{Kind: tickets.LinkTracker, ConversationID: &conv})

			return err
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
moveTo moves the ticket to the state named at the time.
*/
func (d *ticketDemo) moveTo(ctx context.Context, t models.Ticket, state string, at time.Time) error {
	id, ok := d.states[state]
	if !ok {
		return nil
	}

	return d.write(ctx, at, d.mate("move", t.ID), func(ctx context.Context, w *tickets.Writer) error {
		locked, err := w.Q.LockTicket(ctx, models.LockTicketParams{WorkspaceID: t.WorkspaceID, ID: t.ID})
		if err != nil {
			return err
		}

		_, err = w.Apply(ctx, locked, tickets.Change{StateID: &id})

		return err
	})
}

/*
trackerMessages has the first two trackers message everyone linked,
once, and sends the messages at once.
*/
func (d *ticketDemo) trackerMessages(ctx context.Context) error {
	bodies := map[string]string{
		"exports": "Good news: large exports now run in the background and arrive by email when ready. " +
			"Thanks for your patience while we fixed this.",
		"ios-login": "We've found what makes the iOS app crash after sign-in. Version 6.2.1 with the fix is " +
			"waiting for App Store review; we'll write again once it's out.",
	}

	for key, body := range bodies {
		ref, err := d.findDemo(ctx, "demo:Tracker:"+key)
		if err != nil {
			return err
		}

		sent, err := d.app.Actions.Tickets.Updates(ctx, d.owner, ref)
		if err != nil || len(sent) > 0 {
			return err
		}

		plan, err := d.app.Actions.Tickets.SendToLinked(ctx, d.owner, ref, tickets.UpdateInput{Body: body})
		if err != nil {
			return err
		}

		err = d.app.Actions.Tickets.SendUpdate(ctx, jobs.SendTicketUpdateArgs{
			WorkspaceID: d.owner.Workspace.ID, UpdateID: plan.Update.ID,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
findDemo returns the reference of the ticket the demo made under the
external id.
*/
func (d *ticketDemo) findDemo(ctx context.Context, external string) (tickets.Ref, error) {
	t, err := models.New(d.app.DB).FindTicketByExternalID(ctx, models.FindTicketByExternalIDParams{
		WorkspaceID: d.owner.Workspace.ID, ExternalID: &external,
	})

	return tickets.Ref{ID: t.ID}, err
}

/*
demoIncidents lists the running and the resolved incident.
*/
func demoIncidents() []demoTracker {
	return []demoTracker{
		{
			key: "eu-payments", kind: "Incident", title: "Card payments failing for EU customers",
			opened: 5 * time.Hour, reports: 41,
			fields: map[string]any{"severity": "major", "areas": []any{"Payments"}},
			log: []demoPost{
				{after: 10 * time.Minute, body: "We're looking into reports of failed card payments in the EU."},
				{
					after: 2 * time.Hour, state: "Identified",
					body: "A certificate change at our payment provider rejects EU cards. We're working with them.",
				},
				{
					after: 4 * time.Hour, state: "Monitoring",
					body: "A fix is out and payments are going through again. We're watching it closely.",
				},
			},
		},
		{
			key: "email-delays", kind: "Incident", title: "Email notifications delayed",
			opened: 19 * 24 * time.Hour, reports: 26,
			fields: map[string]any{"severity": "minor", "areas": []any{"Email"}},
			log: []demoPost{
				{after: 15 * time.Minute, body: "Some email notifications are arriving up to an hour late."},
				{after: time.Hour, state: "Identified", body: "A queue at our email provider is backed up."},
				{after: 3 * time.Hour, state: "Monitoring", body: "Delayed emails are going out now."},
				{after: 5 * time.Hour, state: "Resolved", body: "All delayed emails went out. Sorry for the wait."},
			},
		},
	}
}

/*
incidents makes the demo's incidents.
*/
func (d *ticketDemo) incidents(ctx context.Context) error {
	for _, spec := range demoIncidents() {
		if err := d.tracker(ctx, spec); err != nil {
			return err
		}
	}

	return nil
}

/*
postLog posts the incident's public log, each post at its time.
*/
func (d *ticketDemo) postLog(ctx context.Context, t models.Ticket, spec demoTracker, opened time.Time) error {
	for i, post := range spec.log {
		in := tickets.PublicUpdateInput{Body: post.body, Source: "dimmi"}
		if id, ok := d.states[post.state]; ok {
			in.StateID = &id
		}

		err := d.write(ctx, opened.Add(post.after), d.mate("post", spec.key, strconv.Itoa(i)),
			func(ctx context.Context, w *tickets.Writer) error {
				locked, err := w.Q.LockTicket(ctx, models.LockTicketParams{WorkspaceID: t.WorkspaceID, ID: t.ID})
				if err != nil {
					return err
				}

				_, err = w.PublicUpdate(ctx, locked, in)

				return err
			})
		if err != nil {
			return err
		}
	}

	return nil
}

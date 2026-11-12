/*
|--------------------------------------------------------------------------
| Report Digests
|--------------------------------------------------------------------------
|
| A teammate can have the overview mailed to them: daily, covering the
| day before, or weekly on Monday, covering the week before, each
| compared with the period before it and sent at eight in the morning
| of the teammate's own time zone, else the workspace's. The digest is
| worked out as the teammate sees the workspace when it goes, so it
| never shows what they may not see; one who left gets nothing.
|
*/

package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The digest schedules, and the hour of the morning they go out.
*/
const (
	DigestDaily  = "daily"
	DigestWeekly = "weekly"
	digestHour   = 8
	digestBatch  = 50
)

/*
Digests manages and sends report digests.
*/
type Digests struct {
	Reports *Reports
	Mail    *mail.Manager
	AppURL  string
}

/*
Digest is a teammate's subscription.
*/
type Digest struct {
	Frequency  string
	Filters    Filters
	NextSendAt time.Time
	LastSentAt *time.Time
}

/*
storedFilters is what a digest keeps of its filters.
*/
type storedFilters struct {
	Inboxes       []ids.UUID `json:"inboxes,omitempty"`
	Teams         []ids.UUID `json:"teams,omitempty"`
	Channels      []string   `json:"channels,omitempty"`
	BusinessHours bool       `json:"business_hours,omitempty"`
}

/*
Show returns the member's digest, and false when they have none.
*/
func (d *Digests) Show(ctx context.Context, m authz.Membership) (Digest, bool, error) {
	var (
		row   models.ReportDigest
		found = true
	)

	err := d.Reports.read(ctx, m, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).FindMemberReportDigest(ctx, models.FindMemberReportDigestParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID,
		})
		if errors.Is(err, database.ErrNotFound) {
			found = false

			return nil
		}

		return err
	})
	if err != nil || !found {
		return Digest{}, false, err
	}

	return digestOf(row), true, nil
}

/*
Save subscribes the member, or changes their digest.
*/
func (d *Digests) Save(ctx context.Context, m authz.Membership, frequency string, f Filters) (Digest, error) {
	if frequency != DigestDaily && frequency != DigestWeekly {
		return Digest{}, exceptions.ErrReportDigestFrequency
	}

	filters, err := json.Marshal(storedFilters{
		Inboxes: f.Inboxes, Teams: f.Teams, Channels: f.Channels, BusinessHours: f.BusinessHours,
	})
	if err != nil {
		return Digest{}, fmt.Errorf("encode digest filters: %w", err)
	}

	now := d.Reports.Clock.Now()

	var row models.ReportDigest

	err = d.Reports.read(ctx, m, func(ctx context.Context, tx *database.Tx) error {
		row, err = models.New(tx).SaveReportDigest(ctx, models.SaveReportDigestParams{
			ID: ids.New(), WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, Frequency: frequency,
			Filters: filters, NextSendAt: nextSend(frequency, now, d.location(ctx, tx, m)), Now: now,
		})

		return err
	})

	return digestOf(row), err
}

/*
Delete stops the member's digest.
*/
func (d *Digests) Delete(ctx context.Context, m authz.Membership) error {
	return d.Reports.read(ctx, m, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).DeleteMemberReportDigest(ctx, models.DeleteMemberReportDigestParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID,
		})

		return err
	})
}

/*
digestOf reads a stored digest.
*/
func digestOf(row models.ReportDigest) Digest {
	var s storedFilters
	_ = json.Unmarshal(row.Filters, &s)

	return Digest{
		Frequency: row.Frequency, NextSendAt: row.NextSendAt, LastSentAt: row.LastSentAt,
		Filters: Filters{Inboxes: s.Inboxes, Teams: s.Teams, Channels: s.Channels, BusinessHours: s.BusinessHours},
	}
}

/*
location returns the member's own time zone, else the workspace's.
*/
func (d *Digests) location(ctx context.Context, tx *database.Tx, m authz.Membership) *time.Location {
	if user, err := models.New(tx).FindUser(ctx, m.Member.UserID); err == nil && user.Timezone != nil {
		if loc, err := time.LoadLocation(*user.Timezone); err == nil {
			return loc
		}
	}

	if loc, err := time.LoadLocation(m.Workspace.Timezone); err == nil {
		return loc
	}

	return time.UTC
}

/*
nextSend returns the next eight in the morning after now, on a Monday
for a weekly digest.
*/
func nextSend(frequency string, now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), digestHour, 0, 0, 0, loc)

	for !next.After(now) || (frequency == DigestWeekly && next.Weekday() != time.Monday) {
		next = next.AddDate(0, 0, 1)
	}

	return next
}

/*
SendDigests sends the digests that are due, a batch at a time.
*/
func (d *Digests) SendDigests(ctx context.Context) error {
	now := d.Reports.Clock.Now()

	due, err := models.New(d.Reports.DB).ReportDigestsDue(ctx, models.ReportDigestsDueParams{
		At: now, Batch: digestBatch,
	})
	if err != nil {
		return fmt.Errorf("find due digests: %w", err)
	}

	var errs []error
	for _, row := range due {
		errs = append(errs, d.send(ctx, row.WorkspaceID, row.ID))
	}

	return errors.Join(errs...)
}

/*
send works out one digest as its teammate sees the workspace now, queues
the mail and moves the digest on.
*/
func (d *Digests) send(ctx context.Context, ws, id ids.UUID) error {
	ctx = tenancy.With(ctx, ws.String())

	return d.Reports.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := q.FindReportDigest(ctx, models.FindReportDigestParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return fmt.Errorf("find digest: %w", err)
		}

		to, err := digestReader(ctx, q, row)
		if err != nil {
			return d.moveOn(ctx, q, row, to.m)
		}

		if err := d.queue(ctx, tx, to, row); err != nil {
			return err
		}

		return d.moveOn(ctx, q, row, to.m)
	})
}

/*
moveOn records the send and schedules the next.
*/
func (d *Digests) moveOn(ctx context.Context, q *models.Queries, row models.ReportDigest, m authz.Membership) error {
	now := d.Reports.Clock.Now()
	loc := time.UTC
	if l, err := time.LoadLocation(m.Workspace.Timezone); err == nil {
		loc = l
	}

	return q.MarkReportDigestSent(ctx, models.MarkReportDigestSentParams{
		Now: now, NextSendAt: nextSend(row.Frequency, now, loc), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
}

/*
digestRecipient is the teammate a digest goes to: their membership as
it stands now and their user.
*/
type digestRecipient struct {
	m    authz.Membership
	user models.User
}

/*
digestReader returns the membership of the digest's teammate as it
stands now, refusing one who left or may not read conversations.
*/
func digestReader(ctx context.Context, q *models.Queries, row models.ReportDigest) (digestRecipient, error) {
	ws, err := q.FindWorkspace(ctx, row.WorkspaceID)
	if err != nil {
		return digestRecipient{}, fmt.Errorf("find workspace: %w", err)
	}

	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: ws.ID, ID: row.MemberID})
	if err != nil {
		return digestRecipient{m: authz.Membership{Workspace: ws}}, fmt.Errorf("find member: %w", err)
	}

	m := authz.NewMembership(ws, member, auth.RolePermissions(enums.Role(member.Role)))
	if member.Status != string(enums.MemberStatusActive) || !m.Can("conversations:read") {
		return digestRecipient{m: m}, exceptions.ErrForbidden
	}

	if m.AccessTeams, err = authz.LoadAccessTeams(ctx, q, member); err != nil {
		return digestRecipient{m: m}, err
	}

	if m.HiddenInboxes, err = authz.LoadHiddenInboxes(ctx, q, member); err != nil {
		return digestRecipient{m: m}, err
	}

	user, err := q.FindUser(ctx, member.UserID)

	return digestRecipient{m: m, user: user}, err
}

/*
queue works out the overview for the period and queues the mail.
*/
func (d *Digests) queue(ctx context.Context, tx *database.Tx, to digestRecipient, row models.ReportDigest) error {
	m := to.m
	f := digestOf(row).Filters
	now := d.Reports.Clock.Now()
	loc := time.UTC
	if l, err := time.LoadLocation(m.Workspace.Timezone); err == nil {
		loc = l
	}

	days := 1
	if row.Frequency == DigestWeekly {
		days = 7
	}

	yesterday := midnight(now.In(loc)).AddDate(0, 0, -1)
	f.From, f.To, f.Compare = yesterday.AddDate(0, 0, 1-days).Format(time.DateOnly),
		yesterday.Format(time.DateOnly), true

	q, err := NewQuery(m, f, now)
	if err != nil {
		return err
	}

	var report Report
	if err := report.fill(ctx, tx, Overview{}, q); err != nil {
		return err
	}

	link := strings.TrimRight(d.AppURL, "/") + "/d/" + m.Workspace.Slug + "/reports?from=" + f.From + "&to=" + f.To

	letter := appmail.ReportDigestMail{
		Name: to.user.Name, Workspace: m.Workspace.Name, Weekly: row.Frequency == DigestWeekly,
		From: f.From, To: f.To, Lines: DigestLines(report.Metrics), ReportURL: link,
	}

	if err := appmail.ToUser(d.Mail, to.user).QueueTx(ctx, tx, letter); err != nil {
		return fmt.Errorf("queue digest mail: %w", err)
	}

	return nil
}

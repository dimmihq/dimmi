/*
|--------------------------------------------------------------------------
| Series Journeys Controller
|--------------------------------------------------------------------------
|
| The people on a series: list them by step and status, add contacts or
| companies by hand, read one journey step by step, take someone off,
| try the series on yourself and skip the waits of that test. Also the
| series' report, picking the winner of a split, and the journeys of one
| contact or company.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SeriesEnrollmentsOutput is a page of journeys.
*/
type SeriesEnrollmentsOutput struct {
	Body fhttp.Collection[resources.SeriesEnrollment]
}

/*
SeriesTraceOutput is one journey and its steps.
*/
type SeriesTraceOutput struct {
	Body fhttp.Resource[resources.SeriesTrace]
}

/*
SeriesReportOutput is a series' report.
*/
type SeriesReportOutput struct {
	Body fhttp.Resource[resources.SeriesReport]
}

/*
Enrollments lists a series' journeys.
*/
func (c *SeriesController) Enrollments(ctx context.Context, in *requests.SeriesEnrollmentsRequest,
) (*SeriesEnrollmentsOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}

	rows, err := c.Series.Journeys(ctx, series.JourneysInput{
		Membership: membership(ctx), ID: id, NodeID: textOrNil(in.Node), Status: textOrNil(in.Status),
		Limit: limit, Offset: in.Offset,
	})
	if err != nil {
		return nil, err
	}

	items := make([]resources.SeriesEnrollment, len(rows))
	for i, r := range rows {
		items[i] = resources.NewSeriesJourney(r)
	}

	page := pagination.Page[resources.SeriesEnrollment]{Items: items, Limit: limit}

	return &SeriesEnrollmentsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Enroll adds contacts or companies to a live series.
*/
func (c *SeriesController) Enroll(ctx context.Context, in *requests.EnrollSeriesRequest,
) (*SeriesEnrollmentsOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	add := series.EnrollInput{Membership: membership(ctx), ID: id, By: "manual"}
	if add.ContactIDs, err = decodeAll(c.IDs, models.PrefixContact, in.Body.ContactIDs); err != nil {
		return nil, err
	}

	if add.CompanyIDs, err = decodeAll(c.IDs, models.PrefixCompany, in.Body.CompanyIDs); err != nil {
		return nil, err
	}

	if p := caller(ctx); p != nil && p.Kind == "token" {
		add.By = "api"
	}

	return journeysOutput(c.Series.Enroll(ctx, add))
}

/*
Try enrolls the teammate's own contact as a test journey.
*/
func (c *SeriesController) Try(ctx context.Context, in *requests.SeriesRequest) (*SeriesTraceOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	e, err := c.Series.TrySelf(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &SeriesTraceOutput{Body: fhttp.NewResource(resources.NewSeriesTrace(e, nil))}, nil
}

/*
Trace reads one journey step by step.
*/
func (c *SeriesController) Trace(ctx context.Context, in *requests.SeriesEnrollmentRequest) (*SeriesTraceOutput,
	error,
) {
	id, journey, err := c.journey(in)
	if err != nil {
		return nil, err
	}

	e, steps, err := c.Series.Trace(ctx, membership(ctx), id, journey)
	if err != nil {
		return nil, err
	}

	return &SeriesTraceOutput{Body: fhttp.NewResource(resources.NewSeriesTrace(e, steps))}, nil
}

/*
Remove takes someone off a series.
*/
func (c *SeriesController) Remove(ctx context.Context, in *requests.SeriesEnrollmentRequest) (*EmptyOutput, error) {
	id, journey, err := c.journey(in)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Series.Remove(ctx, membership(ctx), id, journey)
}

/*
Skip ends the wait of a test journey now.
*/
func (c *SeriesController) Skip(ctx context.Context, in *requests.SeriesEnrollmentRequest) (*EmptyOutput, error) {
	id, journey, err := c.journey(in)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Series.Skip(ctx, membership(ctx), id, journey)
}

/*
Report reads a series' numbers.
*/
func (c *SeriesController) Report(ctx context.Context, in *requests.SeriesReportRequest) (*SeriesReportOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	days := in.Days
	if days <= 0 {
		days = 30
	}

	r, err := c.Series.Report(ctx, membership(ctx), id, days)
	if err != nil {
		return nil, err
	}

	return &SeriesReportOutput{Body: fhttp.NewResource(resources.NewSeriesReport(r))}, nil
}

/*
Winner sends everyone who reaches a split from now on down one variant.
*/
func (c *SeriesController) Winner(ctx context.Context, in *requests.SeriesWinnerRequest) (*SeriesOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	return c.output(ctx)(c.Series.PickWinner(ctx, membership(ctx), series.WinnerInput{
		ID: id, Node: in.Node, Path: in.Body.Path,
	}))
}

/*
ContactJourneys lists the journeys of a contact.
*/
func (c *SeriesController) ContactJourneys(ctx context.Context, in *requests.ContactSeriesRequest,
) (*SeriesEnrollmentsOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return personOutput(c.Series.PersonJourneys(ctx, membership(ctx), &id, nil))
}

/*
CompanyJourneys lists the journeys of a company.
*/
func (c *SeriesController) CompanyJourneys(ctx context.Context, in *requests.CompanySeriesRequest,
) (*SeriesEnrollmentsOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCompany), in.Company)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return personOutput(c.Series.PersonJourneys(ctx, membership(ctx), nil, &id))
}

/*
journey reads the series and journey a path names.
*/
func (c *SeriesController) journey(in *requests.SeriesEnrollmentRequest) (ids.UUID, ids.UUID, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return id, id, err
	}

	journey, err := c.IDs.Decode(string(models.PrefixSeriesEnrollment), in.Enrollment)
	if err != nil {
		return id, id, exceptions.ErrNotFound
	}

	return id, journey, nil
}

/*
journeysOutput answers with the journeys started.
*/
func journeysOutput(rows []models.SeriesEnrollment, err error) (*SeriesEnrollmentsOutput, error) {
	if err != nil {
		return nil, err
	}

	items := make([]resources.SeriesEnrollment, len(rows))
	for i, e := range rows {
		items[i] = resources.NewSeriesEnrollment(e)
	}

	page := pagination.Page[resources.SeriesEnrollment]{Items: items, Limit: max(len(items), 1)}

	return &SeriesEnrollmentsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
personOutput answers with a person's journeys.
*/
func personOutput(rows []models.ListContactSeriesEnrollmentsRow, err error) (*SeriesEnrollmentsOutput, error) {
	if err != nil {
		return nil, err
	}

	items := make([]resources.SeriesEnrollment, len(rows))
	for i, r := range rows {
		items[i] = resources.NewSeriesEnrollment(models.SeriesEnrollment{
			ID: r.ID, SeriesID: r.SeriesID, ContactID: r.ContactID, CompanyID: r.CompanyID, Status: r.Status,
			NodeID: r.NodeID, WakeAt: r.WakeAt, EnteredAt: r.EnteredAt, EnteredBy: r.EnteredBy, GoalAt: r.GoalAt,
			LeftAt: r.LeftAt, EndReason: r.EndReason, Test: r.Test,
		})
		items[i].SeriesName = r.SeriesName
	}

	page := pagination.Page[resources.SeriesEnrollment]{Items: items, Limit: max(len(items), 1)}

	return &SeriesEnrollmentsOutput{Body: fhttp.NewCollection(page)}, nil
}

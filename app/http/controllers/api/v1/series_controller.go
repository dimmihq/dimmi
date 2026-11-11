/*
|--------------------------------------------------------------------------
| Series Controller
|--------------------------------------------------------------------------
|
| GET /series lists series; POST starts a draft; GET one reads it; PATCH
| saves the working copy and settings over the lock version read; DELETE
| removes a draft that never went live. A series is checked, published,
| paused, resumed, archived, copied, and an old version restored into
| the working copy.
|
*/

package v1

import (
	"context"
	"encoding/json"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SeriesController serves series.
*/
type SeriesController struct {
	Series *series.Series
	IDs    *ids.Registry
}

/*
Index lists series, most recently changed first.
*/
func (c *SeriesController) Index(ctx context.Context, in *requests.ListSeriesRequest) (*SeriesListOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}

	rows, err := c.Series.List(ctx, series.ListInput{
		Membership: membership(ctx), Status: textOrNil(in.Status), Search: textOrNil(in.Search), Limit: limit,
		Offset: in.Offset,
	})
	if err != nil {
		return nil, err
	}

	items := make([]resources.Series, len(rows))
	for i, r := range rows {
		items[i] = resources.NewSeries(r, c.Series.LiveHash(ctx, r))
		t := c.Series.Totals(ctx, r)
		items[i].Totals = &resources.SeriesTotals{
			Entered: t.Entered, Active: t.Active, Finished: t.Finished, Exited: t.Exited, Goal: t.Goal,
		}
	}

	return &SeriesListOutput{Body: fhttp.NewCollection(pagination.Page[resources.Series]{Items: items, Limit: limit})},
		nil
}

/*
Store starts a draft, empty or from a definition.
*/
func (c *SeriesController) Store(ctx context.Context, in *requests.StoreSeriesRequest) (*SeriesOutput, error) {
	var spec *series.Spec

	if len(in.Body.Spec) > 0 {
		parsed, err := parseSpec(in.Body.Spec)
		if err != nil {
			return nil, err
		}

		spec = &parsed
	}

	return c.output(ctx)(c.Series.Create(ctx, series.CreateInput{
		Membership: membership(ctx), Name: in.Body.Name, Subject: in.Body.Subject, Spec: spec,
	}))
}

/*
Show reads one series.
*/
func (c *SeriesController) Show(ctx context.Context, in *requests.SeriesRequest) (*SeriesOutput, error) {
	return c.act(ctx, in.Series, c.Series.Show)
}

/*
Update saves the working copy and settings.
*/
func (c *SeriesController) Update(ctx context.Context, in *requests.UpdateSeriesRequest) (*SeriesOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	save, err := c.saveInput(in.Body)
	if err != nil {
		return nil, err
	}

	save.Membership, save.ID = membership(ctx), id

	return c.output(ctx)(c.Series.Save(ctx, save))
}

/*
Destroy removes a draft that never went live.
*/
func (c *SeriesController) Destroy(ctx context.Context, in *requests.SeriesRequest) (*EmptyOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Series.Delete(ctx, membership(ctx), id)
}

/*
Check lists what the series misses before it can go live.
*/
func (c *SeriesController) Check(ctx context.Context, in *requests.CheckSeriesRequest) (*SeriesProblemsOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	row, err := c.Series.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	spec, err := series.SpecOf(row)
	if len(in.Body.Spec) > 0 {
		spec, err = parseSpec(in.Body.Spec)
	}

	if err != nil {
		return nil, err
	}

	problems := series.Check(spec, row.Subject)
	if problems == nil {
		problems = []series.Problem{}
	}

	return &SeriesProblemsOutput{Body: fhttp.NewResource(SeriesProblems{Object: "series_check", Problems: problems})},
		nil
}

/*
Publish makes the working copy the live version.
*/
func (c *SeriesController) Publish(ctx context.Context, in *requests.PublishSeriesRequest) (*SeriesOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	return c.output(ctx)(c.Series.Publish(ctx, series.PublishInput{
		Membership: membership(ctx), ID: id, Note: in.Body.Note, Moves: in.Body.Moves,
	}))
}

/*
Pause holds a live series.
*/
func (c *SeriesController) Pause(ctx context.Context, in *requests.SeriesRequest) (*SeriesOutput, error) {
	return c.act(ctx, in.Series, c.Series.Pause)
}

/*
Resume sets a paused series live again.
*/
func (c *SeriesController) Resume(ctx context.Context, in *requests.SeriesRequest) (*SeriesOutput, error) {
	return c.act(ctx, in.Series, c.Series.Resume)
}

/*
Archive ends a series and every journey on it.
*/
func (c *SeriesController) Archive(ctx context.Context, in *requests.SeriesRequest) (*SeriesOutput, error) {
	return c.act(ctx, in.Series, c.Series.Archive)
}

/*
Duplicate copies a series into a new draft.
*/
func (c *SeriesController) Duplicate(ctx context.Context, in *requests.SeriesRequest) (*SeriesOutput, error) {
	return c.act(ctx, in.Series, c.Series.Duplicate)
}

/*
Versions lists a series' published versions.
*/
func (c *SeriesController) Versions(ctx context.Context, in *requests.SeriesRequest) (*SeriesVersionsOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	rows, err := c.Series.Versions(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	items := make([]resources.SeriesVersion, len(rows))
	for i, v := range rows {
		items[i] = resources.NewSeriesVersion(v)
	}

	page := pagination.Page[resources.SeriesVersion]{Items: items, Limit: max(len(items), 1)}

	return &SeriesVersionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Restore copies an old version into the working copy.
*/
func (c *SeriesController) Restore(ctx context.Context, in *requests.SeriesVersionRequest) (*SeriesOutput, error) {
	id, err := c.decode(in.Series)
	if err != nil {
		return nil, err
	}

	version, err := c.IDs.Decode(string(models.PrefixSeriesVersion), in.Version)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return c.output(ctx)(c.Series.Restore(ctx, membership(ctx), id, version))
}

/*
saveInput reads a save's fields.
*/
func (c *SeriesController) saveInput(b requests.SeriesBody) (series.SaveInput, error) {
	in := series.SaveInput{
		LockVersion: b.LockVersion, Name: b.Name, Description: b.Description, Service: b.Service, SlackAs: b.SlackAs,
		AIReplies: b.AIReplies,
	}

	if len(b.Spec) > 0 {
		spec, err := parseSpec(b.Spec)
		if err != nil {
			return in, err
		}

		in.Spec = &spec
	}

	refs := []struct {
		to     **ids.UUID
		from   string
		prefix models.Prefix
	}{
		{&in.TopicID, b.TopicID, models.PrefixBroadcastTopic},
		{&in.EmailAddressID, b.EmailAddressID, models.PrefixEmailAddress},
		{&in.SenderMemberID, b.SenderID, models.PrefixMember},
		{&in.ReplyTeamID, b.ReplyTeamID, models.PrefixTeam},
		{&in.ReplyMemberID, b.ReplyMemberID, models.PrefixMember},
	}

	for _, r := range refs {
		id, err := optionalID(c.IDs, r.prefix, r.from)
		if err != nil {
			return in, err
		}

		*r.to = id
	}

	return in, nil
}

/*
act runs an action on one series and answers it.
*/
func (c *SeriesController) act(ctx context.Context, public string,
	fn func(context.Context, authz.Membership, ids.UUID) (models.Series, error),
) (*SeriesOutput, error) {
	id, err := c.decode(public)
	if err != nil {
		return nil, err
	}

	return c.output(ctx)(fn(ctx, membership(ctx), id))
}

/*
output answers with one series.
*/
func (c *SeriesController) output(ctx context.Context) func(models.Series, error) (*SeriesOutput, error) {
	return func(row models.Series, err error) (*SeriesOutput, error) {
		if err != nil {
			return nil, err
		}

		return &SeriesOutput{Body: fhttp.NewResource(resources.NewSeries(row, c.Series.LiveHash(ctx, row)))}, nil
	}
}

/*
decode reads a series' public id.
*/
func (c *SeriesController) decode(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixSeries), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}

/*
parseSpec reads a definition sent as JSON.
*/
func parseSpec(raw json.RawMessage) (series.Spec, error) {
	var spec series.Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return series.NewSpec(), series.ErrProblems.With("reason", "The series definition is not valid JSON.")
	}

	return series.Normalize(spec), nil
}

/*
optionalID decodes a public id the request may leave empty.
*/
func optionalID(registry *ids.Registry, prefix models.Prefix, public string) (*ids.UUID, error) {
	if public == "" {
		return nil, nil
	}

	id, err := registry.Decode(string(prefix), public)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return &id, nil
}

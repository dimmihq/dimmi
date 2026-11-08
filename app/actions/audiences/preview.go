/*
|--------------------------------------------------------------------------
| Previewing An Audience
|--------------------------------------------------------------------------
|
| A preview says how many people a rule matches right now, using the
| same evaluator every surface uses, so the number is never a second
| opinion. Membership is never stored, so a preview reads the contacts.
| Up to 50,000 contacts are all read; above that a random sample of
| about 20,000 is read and the count is scaled and marked approximate.
| Page conditions cannot be known without a browser: people who match
| only on some pages are counted apart.
|
*/

package audiences

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
Sizes of a preview.
*/
const (
	exactUpTo   = 50_000
	sampleSize  = 20_000
	pageSize    = 1_000
	maxSample   = 25
	defaultTake = 10
)

/*
PreviewInput is a rule to preview. AudienceID, when set, also stores the
count as the saved audience's last estimate.
*/
type PreviewInput struct {
	Membership authz.Membership
	Rules      json.RawMessage
	AudienceID *ids.UUID
	Sample     int
}

/*
Preview is who a rule matches: Count people match, Pages more match on
some pages only, out of Total contacts. Approximate is set when the
counts were scaled from a sample. Sample lists some people who match,
only to members who may read contacts.
*/
type Preview struct {
	Count       int64
	Pages       int64
	Total       int64
	Approximate bool
	Sample      []models.Contact
	Warnings    []audience.Warning
}

/*
counter is a preview being counted.
*/
type counter struct {
	program *audience.Program
	env     audience.Env
	take    int
	out     *Preview
}

/*
Preview counts who a rule matches.
*/
func (a *Audiences) Preview(ctx context.Context, in PreviewInput) (Preview, error) {
	m := in.Membership
	if !m.Can("audiences:read") {
		return Preview{}, exceptions.ErrForbidden
	}

	parsed, err := Parse(in.Rules)
	if err != nil {
		return Preview{}, err
	}

	program, _ := audience.Compile(parsed, audience.Base(), audience.SavedAudience())
	out := Preview{Warnings: audience.Lint(parsed), Sample: []models.Contact{}}
	c := counter{program: program, out: &out, take: sampleTake(m, in.Sample)}

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		env, err := a.env(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}

		c.env = env
		if err := c.count(ctx, q, m.Workspace.ID); err != nil {
			return err
		}

		return a.keepEstimate(ctx, q, in, out)
	})

	return out, err
}

/*
env is the evaluation environment of a workspace: its time zone, its
saved audiences, and page fields unknown.
*/
func (a *Audiences) env(ctx context.Context, q *models.Queries, ws ids.UUID) (audience.Env, error) {
	resolver, err := audiences.LoadResolver(ctx, q, ws)
	if err != nil {
		return audience.Env{}, err
	}

	workspace, err := q.FindWorkspace(ctx, ws)
	if err != nil {
		return audience.Env{}, fmt.Errorf("find workspace: %w", err)
	}

	loc, err := time.LoadLocation(workspace.Timezone)
	if err != nil {
		loc = time.UTC
	}

	return audience.Env{
		Now: a.Clock.Now(), Location: loc, Saved: resolver.Saved,
		Unknown: func(f audience.Field) bool { return f.Where == audience.Client },
	}, nil
}

/*
count reads the contacts page by page, sampled in a large workspace.
*/
func (c *counter) count(ctx context.Context, q *models.Queries, ws ids.UUID) error {
	total, err := q.CountAudienceContacts(ctx, ws)
	if err != nil {
		return fmt.Errorf("count contacts: %w", err)
	}

	fraction := 1.0
	if total > exactUpTo {
		fraction = float64(sampleSize) / float64(total)
	}

	c.out.Total = total
	after, scanned := ids.Nil(), int64(0)

	for {
		page, err := q.AudienceContactPage(ctx, models.AudienceContactPageParams{
			WorkspaceID: ws, After: after, Fraction: fraction, PageSize: pageSize,
		})
		if err != nil {
			return fmt.Errorf("read contacts: %w", err)
		}

		if err := c.judge(ctx, q, ws, page); err != nil {
			return err
		}

		scanned += int64(len(page))
		if len(page) < pageSize {
			break
		}

		after = page[len(page)-1].ID
	}

	c.scale(scanned, fraction)

	return nil
}

/*
judge evaluates one page of contacts.
*/
func (c *counter) judge(ctx context.Context, q *models.Queries, ws ids.UUID, page []models.Contact) error {
	if len(page) == 0 {
		return nil
	}

	people, err := audiences.LoadPeople(ctx, q, ws, page)
	if err != nil {
		return err
	}

	for _, p := range people {
		env := c.env
		env.Facts = p

		switch t, _ := c.program.Partial(env); t {
		case audience.True:
			c.out.Count++
			if len(c.out.Sample) < c.take {
				c.out.Sample = append(c.out.Sample, p.Contact)
			}
		case audience.Unknown:
			c.out.Pages++
		case audience.False:
		}
	}

	return nil
}

/*
scale turns sampled counts into estimates of the whole workspace.
*/
func (c *counter) scale(scanned int64, fraction float64) {
	if fraction >= 1 || scanned == 0 {
		return
	}

	ratio := float64(c.out.Total) / float64(scanned)
	c.out.Count = int64(math.Round(float64(c.out.Count) * ratio))
	c.out.Pages = int64(math.Round(float64(c.out.Pages) * ratio))
	c.out.Approximate = true
}

/*
keepEstimate stores a saved audience's count for its list.
*/
func (a *Audiences) keepEstimate(ctx context.Context, q *models.Queries, in PreviewInput, p Preview) error {
	if in.AudienceID == nil {
		return nil
	}

	estimate, _ := json.Marshal(map[string]any{
		"count": p.Count, "pages": p.Pages, "approximate": p.Approximate, "at": a.Clock.Now(),
	})

	return wrap("keep audience estimate", q.SetAudienceEstimate(ctx, models.SetAudienceEstimateParams{
		Estimate: estimate, WorkspaceID: in.Membership.Workspace.ID, ID: *in.AudienceID,
	}))
}

/*
sampleTake is how many matching people a preview lists: none to a
member who may not read contacts.
*/
func sampleTake(m authz.Membership, asked int) int {
	switch {
	case !m.Can("contacts:read"):
		return 0
	case asked <= 0:
		return defaultTake
	case asked > maxSample:
		return maxSample
	}

	return asked
}

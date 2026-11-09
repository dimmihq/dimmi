/*
|--------------------------------------------------------------------------
| Freezing An Audience
|--------------------------------------------------------------------------
|
| A saved audience always means who matches now. Sometimes a team wants
| the people who matched at one moment, such as everyone on the old plan
| on the day prices changed: freezing puts a tag on each of them. The
| tag then stays as it is, whoever joins or leaves the audience later,
| and anything that reads tags can use it. Every contact is checked, so a
| workspace too large to check at once is refused rather than sampled.
|
*/

package audiences

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contactlists"
	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
maxFrozen is how many contacts freezing checks at once.
*/
const maxFrozen = 100_000

/*
ErrTooLarge refuses to freeze an audience in a workspace with more
contacts than one freeze checks.
*/
var ErrTooLarge = fault.New(fault.Invalid, "audience_too_large",
	"This workspace has more than 100,000 people, too many to freeze at once.")

/*
FreezeInput is a saved audience to freeze into a tag.
*/
type FreezeInput struct {
	Membership authz.Membership
	ID         ids.UUID
	TagName    string
}

/*
Frozen is what freezing did: the tag, how many matched and how many were
newly tagged.
*/
type Frozen struct {
	Tag     models.Tag
	Matched int
	Tagged  int64
}

/*
Freeze tags everyone the saved audience matches now.
*/
func (a *Audiences) Freeze(ctx context.Context, in FreezeInput) (Frozen, error) {
	m := in.Membership
	if !m.Can("audiences:read") || !m.Can("contacts:write") {
		return Frozen{}, exceptions.ErrForbidden
	}

	name := strings.TrimSpace(in.TagName)
	if name == "" || len([]rune(name)) > 50 {
		return Frozen{}, contactlists.ErrTagName
	}

	var out Frozen
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), m.Workspace.ID, a.Clock.Now()
		row, err := find(ctx, q, ws, in.ID)
		if err != nil {
			return err
		}

		members, err := a.members(ctx, q, row)
		if err != nil {
			return err
		}

		tag, err := contactlists.EnsureTag(ctx, q, contactlists.Tagging{WorkspaceID: ws, Now: now}, name)
		if err != nil {
			return err
		}

		tagged, err := contactlists.Tag(ctx, q, contactlists.Tagging{
			WorkspaceID: ws, Tag: tag.ID, Member: m.Member.ID, Contacts: members, Now: now,
		})
		out = Frozen{Tag: tag, Matched: len(members), Tagged: tagged}

		return err
	})

	return out, err
}

/*
members lists everyone the saved audience matches now, every contact
checked.
*/
func (a *Audiences) members(ctx context.Context, q *models.Queries, row models.Audience) ([]ids.UUID, error) {
	ws := row.WorkspaceID
	total, err := q.CountAudienceContacts(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("count contacts: %w", err)
	}

	if total > maxFrozen {
		return nil, ErrTooLarge
	}

	program, err := audiences.Compile(row)
	if err != nil {
		return nil, ErrInvalid.With("reason", err.Error())
	}

	env, err := a.env(ctx, q, ws)
	if err != nil {
		return nil, err
	}

	var out []ids.UUID
	after := ids.Nil()

	for {
		page, err := q.AudienceContactPage(ctx, models.AudienceContactPageParams{
			WorkspaceID: ws, After: after, Fraction: 1, PageSize: pageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("read contacts: %w", err)
		}

		matched, err := judge{program: program, env: env}.matching(ctx, q, page)
		if err != nil {
			return nil, err
		}

		out = append(out, matched...)
		if len(page) < pageSize {
			return out, nil
		}

		after = page[len(page)-1].ID
	}
}

/*
judge decides who a program matches in its environment.
*/
type judge struct {
	program *audience.Program
	env     audience.Env
}

/*
matching keeps the contacts of a page the program matches.
*/
func (j judge) matching(ctx context.Context, q *models.Queries, page []models.Contact) ([]ids.UUID, error) {
	program, env := j.program, j.env
	if len(page) == 0 {
		return nil, nil
	}

	people, err := audiences.LoadPeople(ctx, q, page[0].WorkspaceID, page)
	if err != nil {
		return nil, err
	}

	var out []ids.UUID
	for _, p := range people {
		p.Now = env.Now
		e := env
		e.Facts = p

		if t, _ := program.Partial(e); t == audience.True {
			out = append(out, p.Contact.ID)
		}
	}

	return out, nil
}

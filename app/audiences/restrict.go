/*
|--------------------------------------------------------------------------
| Who May Read
|--------------------------------------------------------------------------
|
| An article may be kept to a saved audience, and so may each collection
| above it. A reader sees the article only when they are in every one of
| those audiences. The same answer decides what dimmi may answer a person
| from, so dimmi never tells someone what they could not read. Anyone the
| server knows nothing about is a lead with no data: only audiences that
| hold for everyone let them in. A rule that cannot be decided denies.
|
*/

package audiences

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
Restrictions are the audiences each article and collection needs,
collections above included.
*/
type Restrictions struct {
	articles    map[ids.UUID][]ids.UUID
	collections map[ids.UUID][]ids.UUID
}

/*
LoadRestrictions reads which audiences each article and collection of
the workspace needs.
*/
func LoadRestrictions(ctx context.Context, q *models.Queries, ws ids.UUID) (Restrictions, error) {
	cols, err := q.CollectionAudiences(ctx, ws)
	if err != nil {
		return Restrictions{}, fmt.Errorf("list collection audiences: %w", err)
	}

	arts, err := q.ArticleAudiences(ctx, ws)
	if err != nil {
		return Restrictions{}, fmt.Errorf("list article audiences: %w", err)
	}

	byID := map[ids.UUID]models.CollectionAudiencesRow{}
	for _, c := range cols {
		byID[c.ID] = c
	}

	r := Restrictions{articles: map[ids.UUID][]ids.UUID{}, collections: map[ids.UUID][]ids.UUID{}}
	for _, c := range cols {
		r.collections[c.ID] = chain(byID, &c.ID)
	}

	for _, a := range arts {
		need := chain(byID, a.CollectionID)
		if a.AudienceID != nil {
			need = append(need, *a.AudienceID)
		}

		r.articles[a.ID] = need
	}

	return r, nil
}

/*
chain is the audiences of a collection and every collection above it.
*/
func chain(byID map[ids.UUID]models.CollectionAudiencesRow, from *ids.UUID) []ids.UUID {
	var out []ids.UUID
	for depth := 0; from != nil && depth < 16; depth++ {
		c, ok := byID[*from]
		if !ok {
			break
		}

		if c.AudienceID != nil && !slices.Contains(out, *c.AudienceID) {
			out = append(out, *c.AudienceID)
		}

		from = c.ParentID
	}

	return out
}

/*
Needed lists every audience some article or collection needs.
*/
func (r Restrictions) Needed() []ids.UUID {
	var out []ids.UUID
	for _, list := range [2]map[ids.UUID][]ids.UUID{r.articles, r.collections} {
		for _, need := range list {
			for _, id := range need {
				if !slices.Contains(out, id) {
					out = append(out, id)
				}
			}
		}
	}

	return out
}

/*
Article reports whether a reader in the allowed audiences may read the
article.
*/
func (r Restrictions) Article(id ids.UUID, allowed []ids.UUID) bool {
	return within(r.articles[id], allowed)
}

/*
Collection reports whether a reader in the allowed audiences may open
the collection.
*/
func (r Restrictions) Collection(id ids.UUID, allowed []ids.UUID) bool {
	return within(r.collections[id], allowed)
}

func within(need, allowed []ids.UUID) bool {
	for _, id := range need {
		if !slices.Contains(allowed, id) {
			return false
		}
	}

	return true
}

/*
Reader is someone about to read: the workspace, the person, nil for a
reader the server knows nothing about, the audiences that matter and
the time.
*/
type Reader struct {
	WorkspaceID ids.UUID
	Person      *Person
	Needed      []ids.UUID
	Now         time.Time
}

/*
Allowed lists the audiences among those needed that the reader is in.
*/
func Allowed(ctx context.Context, q *models.Queries, r Reader) ([]ids.UUID, error) {
	if len(r.Needed) == 0 {
		return nil, nil
	}

	resolver, err := LoadResolver(ctx, q, r.WorkspaceID)
	if err != nil {
		return nil, err
	}

	env := audience.Env{
		Now: r.Now, Saved: resolver.Saved, Facts: audience.Values{"contact.kind": "lead"},
		Unknown: func(f audience.Field) bool { return f.Where == audience.Client },
	}

	if r.Person != nil {
		p := *r.Person
		p.Now = r.Now
		env.Facts = p
	}

	env = Strict(env)
	var out []ids.UUID
	for _, id := range r.Needed {
		program, ok := resolver.Saved(models.PublicID(models.PrefixAudience, id))
		if ok && program.Evaluate(env) {
			out = append(out, id)
		}
	}

	return out, nil
}

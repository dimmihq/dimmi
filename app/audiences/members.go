/*
|--------------------------------------------------------------------------
| Audience Members
|--------------------------------------------------------------------------
|
| Lists of people can be narrowed to those in saved audiences. A person is
| in an audience when its rule holds for them as a preview would judge
| it: in the workspace's time zone, with page conditions unknown, so
| someone who matches only on some pages is not counted in.
|
*/

package audiences

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
Judge says which saved audiences of one workspace a person is in.
*/
type Judge struct {
	resolver *Resolver
	env      audience.Env
}

/*
NewJudge loads the workspace's saved audiences and time zone.
*/
func NewJudge(ctx context.Context, q *models.Queries, ws ids.UUID, now time.Time) (*Judge, error) {
	resolver, err := LoadResolver(ctx, q, ws)
	if err != nil {
		return nil, err
	}

	workspace, err := q.FindWorkspace(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("find workspace: %w", err)
	}

	loc, err := time.LoadLocation(workspace.Timezone)
	if err != nil {
		loc = time.UTC
	}

	return &Judge{resolver: resolver, env: audience.Env{
		Now: now, Location: loc, Saved: resolver.Saved,
		Unknown: func(f audience.Field) bool { return f.Where == audience.Client },
	}}, nil
}

/*
InAny reports whether the person is in at least one of the audiences.
*/
func (j *Judge) InAny(p Person, audiences []ids.UUID) bool {
	env := j.env
	p.Now = env.Now
	env.Facts = p

	for _, id := range audiences {
		program, ok := j.resolver.Saved(models.PublicID(models.PrefixAudience, id))
		if !ok {
			continue
		}

		if truth, _ := program.Partial(env); truth == audience.True {
			return true
		}
	}

	return false
}

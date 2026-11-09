/*
|--------------------------------------------------------------------------
| Article Ratings
|--------------------------------------------------------------------------
|
| Readers say whether an article helped, and a reader who says no may tell
| the team what was missing. Readers need no account, so each answer is
| kept under a keyed hash of who gave it, a signed-in reader or the
| address they came from: answering again changes that one answer, and a
| comment only ever lands on the commenter's own no. Each rater gets a
| handful of answers and comments an hour, which keeps a script from
| filling the team's view with noise.
|
*/

package helpcenters

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Rating limits: each rater gets this many answers and comments an hour on
a help center, comments are cut at this length, and the team's view
lists this many comments.
*/
const (
	ratingsPerHour   = 30
	commentsPerHour  = 10
	maxRatingComment = 2000
	recentComments   = 100
)

/*
ErrRatingLimited refuses a rater who answered or commented too often in
the last hour; ErrRatingCommentEmpty refuses a comment with no words.
*/
var (
	ErrRatingLimited = fault.New(fault.RateLimited, "rating_limited",
		"You've sent a lot of feedback. Try again in a little while.")
	ErrRatingCommentEmpty = fault.New(fault.Invalid, "rating_comment_empty", "Tell us what was missing.").
				OnField("comment")
)

/*
Ratings keeps readers' answers about articles. Key keys the hash of each
rater, so the stored hash names nobody.
*/
type Ratings struct {
	DB      *database.DB
	Clock   clock.Clock
	Limiter *ratelimit.Limiter
	Key     []byte
}

/*
Rating is a reader's answer about the published article at Slug. Rater
names who answered, such as their address or their contact, and Contact
is the signed-in reader, when there is one.
*/
type Rating struct {
	Center  models.HelpCenter
	Slug    string
	Rater   string
	Contact *ids.UUID
	Helpful bool
	Comment string
}

/*
Rate keeps the reader's answer, changing the one they gave before.
*/
func (r *Ratings) Rate(ctx context.Context, in Rating) error {
	if err := r.allow(ctx, "rate", in, ratingsPerHour); err != nil {
		return err
	}
	return r.onArticle(ctx, in, func(ctx context.Context, q *models.Queries, article ids.UUID, rater string) error {
		_, err := q.RateArticle(ctx, models.RateArticleParams{
			WorkspaceID: in.Center.WorkspaceID, ArticleID: article, RaterHash: rater, ContactID: in.Contact,
			Helpful: in.Helpful, Now: r.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("rate article: %w", err)
		}
		return nil
	})
}

/*
Comment adds what the reader says was missing to their own no. A reader
who did not say no has nothing to comment on, and the comment is
dropped.
*/
func (r *Ratings) Comment(ctx context.Context, in Rating) error {
	text := strings.TrimSpace(in.Comment)
	if text == "" {
		return ErrRatingCommentEmpty
	}
	if err := r.allow(ctx, "comment", in, commentsPerHour); err != nil {
		return err
	}
	text = truncate(text, maxRatingComment)
	return r.onArticle(ctx, in, func(ctx context.Context, q *models.Queries, article ids.UUID, rater string) error {
		_, err := q.CommentOnArticleRating(ctx, models.CommentOnArticleRatingParams{
			Comment: &text, Now: r.Clock.Now(), WorkspaceID: in.Center.WorkspaceID, ArticleID: article,
			RaterHash: rater,
		})
		if err != nil {
			return fmt.Errorf("comment on article rating: %w", err)
		}
		return nil
	})
}

/*
allow counts one more answer or comment from the rater on the help
center, refusing it past the hour's limit.
*/
func (r *Ratings) allow(ctx context.Context, what string, in Rating, limit int) error {
	key := "article_rating:" + what + ":" + in.Center.ID.String() + ":" + r.hash(in.Rater)
	res, err := r.Limiter.Attempt(ctx, key, limit, time.Hour)
	if err != nil {
		return fmt.Errorf("limit article ratings: %w", err)
	}
	if !res.Allowed {
		return ErrRatingLimited
	}
	return nil
}

/*
onArticle runs fn on the published article at the slug, with the
rater's hash for it. An article that is not published is quietly left
alone, since readers only ever rate what they read.
*/
func (r *Ratings) onArticle(ctx context.Context, in Rating,
	fn func(context.Context, *models.Queries, ids.UUID, string) error,
) error {
	scoped := tenancy.With(ctx, in.Center.WorkspaceID.String())
	return r.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := q.FindPublishedArticleBySlug(ctx, models.FindPublishedArticleBySlugParams{
			WorkspaceID: in.Center.WorkspaceID, HelpCenterID: in.Center.ID, Slug: in.Slug,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find rated article: %w", err)
		}
		return fn(ctx, q, row.ID, r.hash(row.ID.String()+"|"+in.Rater))
	})
}

func (r *Ratings) hash(value string) string {
	return hex.EncodeToString(hashing.HMAC(r.Key, []byte(value)))
}

/*
ArticleFeedback is what readers said about an article: how many found it
helpful or not, how many left a comment, and the latest comments.
*/
type ArticleFeedback struct {
	Helpful   int
	Unhelpful int
	Comments  int
	Recent    []models.ListArticleRatingCommentsRow
}

/*
For returns what readers said about the article, to the team members
who may read articles.
*/
func (r *Ratings) For(ctx context.Context, m authz.Membership, article ids.UUID) (ArticleFeedback, error) {
	if !m.Can("articles:read") {
		return ArticleFeedback{}, exceptions.ErrForbidden
	}
	var out ArticleFeedback
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findArticle(ctx, q, m.Workspace.ID, article); err != nil {
			return err
		}
		counts, err := q.CountArticleRatings(ctx, models.CountArticleRatingsParams{
			WorkspaceID: m.Workspace.ID, ArticleID: article,
		})
		if err != nil {
			return fmt.Errorf("count article ratings: %w", err)
		}
		out.Helpful, out.Unhelpful, out.Comments = int(counts.Helpful), int(counts.Unhelpful), int(counts.Comments)
		out.Recent, err = q.ListArticleRatingComments(ctx, models.ListArticleRatingCommentsParams{
			WorkspaceID: m.Workspace.ID, ArticleID: article, Lim: recentComments,
		})
		if err != nil {
			return fmt.Errorf("list article rating comments: %w", err)
		}
		return nil
	})
	return out, err
}

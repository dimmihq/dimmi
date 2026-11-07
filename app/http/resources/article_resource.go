/*
|--------------------------------------------------------------------------
| Article Resources
|--------------------------------------------------------------------------
|
| How an article's working copy reads over the API, with where it
| stands: draft, published, or published with changes not yet live.
| Lists leave the body out; one article carries it.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Article is an article's working copy. Body is empty in lists.
*/
type Article struct {
	ID                    string     `json:"id"`
	Object                string     `json:"object"`
	HelpCenterID          string     `json:"help_center_id"`
	CollectionID          *string    `json:"collection_id"`
	Title                 string     `json:"title"`
	Slug                  string     `json:"slug"`
	Description           *string    `json:"description"`
	BodyMarkdown          *string    `json:"body_markdown,omitempty"`
	Status                string     `json:"status"`
	Visibility            string     `json:"visibility"`
	HasUnpublishedChanges bool       `json:"has_unpublished_changes"`
	LockVersion           int32      `json:"lock_version"`
	WordCount             int32      `json:"word_count"`
	PublishedAt           *time.Time `json:"published_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

/*
NewArticle shapes one article with its body.
*/
func NewArticle(a models.Article) Article {
	out := articleSummary(a)
	out.BodyMarkdown = &a.BodyMarkdown
	return out
}

/*
articleSummary shapes an article without its body, as lists show it.
*/
func articleSummary(a models.Article) Article {
	out := Article{
		ID: models.PublicID(models.PrefixArticle, a.ID), Object: "article",
		HelpCenterID: models.PublicID(models.PrefixHelpCenter, a.HelpCenterID), Title: a.Title, Slug: a.Slug,
		Description: a.Description, Status: a.Status, Visibility: a.Visibility,
		HasUnpublishedChanges: a.HasUnpublishedChanges && a.Status == "published", LockVersion: a.LockVersion,
		WordCount: a.WordCount, PublishedAt: a.PublishedAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
	if a.CollectionID != nil {
		out.CollectionID = new(models.PublicID(models.PrefixCollection, *a.CollectionID))
	}
	return out
}

/*
NewArticles shapes a list of articles without bodies.
*/
func NewArticles(rows []models.Article) []Article {
	out := make([]Article, len(rows))
	for i, r := range rows {
		out[i] = articleSummary(r)
	}
	return out
}

/*
ArticleVersion is one frozen version of an article.
*/
type ArticleVersion struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Number    int32     `json:"number"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

/*
NewArticleVersions shapes an article's versions.
*/
func NewArticleVersions(rows []models.ListArticleVersionsRow) []ArticleVersion {
	out := make([]ArticleVersion, len(rows))
	for i, r := range rows {
		out[i] = ArticleVersion{
			ID: models.PublicID(models.PrefixArticleVersion, r.ID), Object: "article_version", Number: r.Number,
			Kind: r.Kind, Title: r.Title, CreatedAt: r.CreatedAt,
		}
	}
	return out
}

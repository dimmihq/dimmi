/*
|--------------------------------------------------------------------------
| Article Feedback Resource
|--------------------------------------------------------------------------
|
| What readers said about an article, as the dashboard shows it beside
| the article: how many found it helpful or not, and the notes readers
| left after a no, with who left them when they were signed in.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ArticleFeedback counts an article's answers and lists its latest notes.
*/
type ArticleFeedback struct {
	Object    string            `json:"object"`
	Helpful   int               `json:"helpful"`
	Unhelpful int               `json:"unhelpful"`
	Comments  int               `json:"comments"`
	Recent    []ArticleFeedNote `json:"recent"`
}

/*
ArticleFeedNote is one note a reader left after saying no, and the
contact who left it when they were signed in.
*/
type ArticleFeedNote struct {
	Comment   string       `json:"comment"`
	Contact   *NoteContact `json:"contact"`
	CreatedAt time.Time    `json:"created_at"`
}

/*
NoteContact names the signed-in reader who left a note.
*/
type NoteContact struct {
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

/*
NewArticleFeedback shapes what readers said about an article.
*/
func NewArticleFeedback(f helpcenters.ArticleFeedback) ArticleFeedback {
	out := ArticleFeedback{
		Object: "article_feedback", Helpful: f.Helpful, Unhelpful: f.Unhelpful, Comments: f.Comments,
		Recent: make([]ArticleFeedNote, len(f.Recent)),
	}
	for i, n := range f.Recent {
		out.Recent[i] = ArticleFeedNote{Comment: n.Comment, CreatedAt: n.UpdatedAt}
		if n.ContactID != nil {
			out.Recent[i].Contact = &NoteContact{
				ID: models.PublicID(models.PrefixContact, *n.ContactID), Name: n.ContactName, Email: n.ContactEmail,
			}
		}
	}
	return out
}

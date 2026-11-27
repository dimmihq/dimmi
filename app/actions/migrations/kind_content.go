/*
|--------------------------------------------------------------------------
| Help Centers, Saved Replies And Schedules
|--------------------------------------------------------------------------
|
| A help center from the other tool lands in the workspace's help center
| of the same name, or its default one while that one is still empty,
| or a new one. Collections nest under their parents, folded at the
| deepest level the help center allows. Articles keep their author,
| their times and their old addresses, which redirect to them. Pictures
| a client uploaded are copied into the help center and their links
| rewritten. A snippet becomes an article only teammates see. Saved
| replies keep their actions, schedules their hours and days off.
|
*/

package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
fileLink is a picture a client uploaded, named in a body as
dimmi-file:<file id>.
*/
var fileLink = regexp.MustCompile(`dimmi-file:([a-z]+_[0-9a-z]+)`)

/*
Help centers.
*/
func (h *HelpCenter) validate() error { return required("name", h.Name) }

func (h *HelpCenter) refs() []Ref { return nil }

func (h *HelpCenter) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: h.Name}

	if len(h.Locales) > 1 {
		out.unsupported("locales")
	}

	found, ok, err := s.existingHelpCenter(ctx, h.Name)
	if err != nil || ok {
		out.id = found

		return out, err
	}

	made, err := s.kit().HelpCenters.Create(ctx, helpcenters.NewHelpCenter{
		Membership: s.runner.m, Name: cut(strings.TrimSpace(h.Name), 80), Locale: h.DefaultLocale,
	})
	if err != nil {
		return out, err
	}

	out.id, out.created = made.ID, true

	return out, nil
}

/*
existingHelpCenter returns the help center this record made before, else
the workspace's one with the name, else its default one while it has no
articles, and reports whether there is one.
*/
func (s *step) existingHelpCenter(ctx context.Context, name string) (ids.UUID, bool, error) {
	if s.prev != nil && s.prev.RecordID != nil {
		if _, err := s.q.FindHelpCenter(ctx, models.FindHelpCenterParams{
			WorkspaceID: s.ws(), ID: *s.prev.RecordID,
		}); err == nil {
			return *s.prev.RecordID, true, nil
		}
	}

	centers, err := s.q.ListHelpCenters(ctx, s.ws())
	if err != nil {
		return ids.Nil(), false, fmt.Errorf("list help centers: %w", err)
	}

	for _, c := range centers {
		if strings.EqualFold(c.Name, strings.TrimSpace(name)) {
			return c.ID, true, nil
		}
	}

	i := slices.IndexFunc(centers, func(c models.HelpCenter) bool { return c.IsDefault })
	if i < 0 {
		return ids.Nil(), false, nil
	}

	n, err := s.q.CountHelpCenterArticles(ctx, models.CountHelpCenterArticlesParams{
		WorkspaceID: s.ws(), HelpCenterID: centers[i].ID,
	})
	if err != nil {
		return ids.Nil(), false, fmt.Errorf("count help center articles: %w", err)
	}

	return centers[i].ID, n == 0, nil
}

/*
Collections.
*/
func (c *Collection) validate() error {
	if err := required("help_center_ref", c.HelpCenterRef); err != nil {
		return err
	}

	return required("name", c.Name)
}

func (c *Collection) refs() []Ref {
	return append(refsOf(KindHelpCenter, c.HelpCenterRef), refsOf(KindCollection, c.ParentRef)...)
}

func (c *Collection) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: c.Name}

	if len(c.Translations) > 0 {
		out.unsupported("translations")
	}

	center, err := s.refs.must("help_center_ref", KindHelpCenter, c.HelpCenterRef)
	if err != nil {
		return out, err
	}

	parent, name, err := s.foldedParent(ctx, c)
	if err != nil {
		return out, err
	}

	if s.prev != nil && s.prev.RecordID != nil {
		moved, err := s.q.MigrateCollection(ctx, models.MigrateCollectionParams{
			Name: cut(name, 80), Description: orNil(cut(c.Description, 300)), ParentID: parent, Now: s.now,
			WorkspaceID: s.ws(), ID: *s.prev.RecordID,
		})
		if err == nil {
			out.id = moved.ID

			return out, nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return out, fmt.Errorf("move collection: %w", err)
		}
	}

	out.id, err = s.kit().Collections.ImportCollection(ctx, s.tx, helpcenters.ImportCollectionInput{
		WorkspaceID: s.ws(), HelpCenterID: center, ParentID: parent, Name: name, Description: c.Description,
	})
	out.created = err == nil

	return out, err
}

/*
foldedParent returns where the collection goes: under its parent, or,
when the parent is already as deep as collections go, beside it with
both names.
*/
func (s *step) foldedParent(ctx context.Context, c *Collection) (*ids.UUID, string, error) {
	if c.ParentRef == "" {
		return nil, c.Name, nil
	}

	parentID, ok := s.refs.id(KindCollection, c.ParentRef)
	if !ok {
		return nil, c.Name, nil
	}

	chain := []models.Collection{}
	at := &parentID

	for at != nil && len(chain) <= helpcenters.MaxCollectionDepth {
		row, err := s.q.FindCollection(ctx, models.FindCollectionParams{WorkspaceID: s.ws(), ID: *at})
		if err != nil {
			return nil, c.Name, fmt.Errorf("find parent collection: %w", err)
		}

		chain = append(chain, row)
		at = row.ParentID
	}

	if len(chain) < helpcenters.MaxCollectionDepth {
		return &parentID, c.Name, nil
	}

	return chain[0].ParentID, chain[0].Name + " / " + c.Name, nil
}

/*
Articles.
*/
func (a *Article) validate() error {
	if err := required("help_center_ref", a.HelpCenterRef); err != nil {
		return err
	}

	if err := required("title", a.Title); err != nil {
		return err
	}

	if err := oneOf("state", a.State, "published", "draft"); err != nil {
		return err
	}

	if err := oneOf("visibility", a.Visibility, "public", "agent_only"); err != nil {
		return err
	}

	if a.Author != nil {
		return checkAuthor("author", *a.Author)
	}

	return nil
}

func (a *Article) refs() []Ref {
	return append(append(refsOf(KindHelpCenter, a.HelpCenterRef), refsOf(KindCollection, a.CollectionRef)...),
		authorRefs(a.Author)...)
}

func (a *Article) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: a.Title}

	if len(a.Translations) > 0 {
		out.unsupported("translations")
	}

	if len(a.Blocks) > 0 {
		out.unsupported("blocks")
	}

	center, err := s.refs.must("help_center_ref", KindHelpCenter, a.HelpCenterRef)
	if err != nil {
		return out, err
	}

	var collection *ids.UUID
	if id, ok := s.refs.id(KindCollection, a.CollectionRef); ok {
		collection = &id
	}

	visibility := a.Visibility
	if visibility == "" {
		visibility = "public"
	}

	art := articleWrite{
		center: center, collection: collection, title: a.Title, description: a.Description, body: a.Body,
		visibility: visibility, publish: a.State != "draft", author: a.Author, redirects: a.Redirects,
		created: a.CreatedAt, updated: a.UpdatedAt,
	}

	return s.writeArticle(ctx, art, out)
}

/*
articleWrite is an article or snippet on its way into a help center.
*/
type articleWrite struct {
	center                   ids.UUID
	collection               *ids.UUID
	title, description, body string
	visibility               string
	publish                  bool
	author                   *Author
	redirects                []string
	created, updated         *time.Time
}

/*
writeArticle writes the article, its author, times and redirects.
*/
func (s *step) writeArticle(ctx context.Context, a articleWrite, out outcome) (outcome, error) {
	body, missing := s.copyPictures(ctx, a.body)
	if missing > 0 {
		out.warn("body", "file_unavailable", "Pictures whose files were not uploaded to this migration were left out.")
	}

	in := helpcenters.ImportArticleInput{
		WorkspaceID: s.ws(), HelpCenterID: a.center, CollectionID: a.collection, Title: a.title,
		Description: a.description, Body: body, Visibility: a.visibility, Publish: a.publish,
	}

	if s.prev != nil && s.prev.RecordID != nil {
		in.ArticleID = s.prev.RecordID
	}

	art, err := s.kit().Articles.ImportArticle(ctx, s.tx, in)
	if err != nil {
		return out, err
	}

	out.id = art.ID
	out.created = in.ArticleID == nil || *in.ArticleID != art.ID

	params := models.MigrateArticleParams{
		CreatedAt: utc(a.created), PublishedAt: utc(a.created), UpdatedAt: utc(a.updated),
		WorkspaceID: s.ws(), ID: art.ID,
	}

	if member, ok := s.member(a.author); ok {
		params.AuthorMemberID = &member
	}

	if err := s.q.MigrateArticle(ctx, params); err != nil {
		return out, fmt.Errorf("date article: %w", err)
	}

	for _, from := range a.redirects {
		if err := s.kit().Articles.ImportRedirect(ctx, s.tx, art, redirectPath(from)); err != nil {
			return out, err
		}
	}

	return out, nil
}

/*
redirectPath returns the path of an old address, as given when it is
already a path.
*/
func redirectPath(from string) string {
	from = strings.TrimSpace(from)
	if i := strings.Index(from, "://"); i >= 0 {
		rest := from[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			return rest[j:]
		}

		return ""
	}

	return from
}

/*
copyPictures copies every uploaded picture the body names into the help
center and points the body at the copy, and counts the pictures whose
files are not there.
*/
func (s *step) copyPictures(ctx context.Context, body string) (string, int) {
	missing := 0

	return fileLink.ReplaceAllStringFunc(body, func(link string) string {
		id := fileLink.FindStringSubmatch(link)[1]

		url, err := s.keepPicture(ctx, id)
		if err != nil {
			missing++

			return ""
		}

		return url
	}), missing
}

/*
keepPicture copies the uploaded file into the help center's pictures
and returns its address.
*/
func (s *step) keepPicture(ctx context.Context, fileID string) (string, error) {
	row, err := s.file(ctx, fileID)
	if err != nil {
		return "", err
	}

	disk, err := s.kit().Disks.Disk(row.Disk)
	if err != nil {
		return "", fmt.Errorf("open disk: %w", err)
	}

	f, err := disk.Open(ctx, row.Path)
	if err != nil {
		return "", fmt.Errorf("open picture: %w", err)
	}

	defer func() { _ = f.Close() }()

	return s.kit().Images.Keep(ctx, s.ws(), io.LimitReader(f, helpcenters.MaxImageBytes+1))
}

/*
Snippets become articles only teammates see, in the help center named
or the default one.
*/
func (n *Snippet) validate() error {
	if err := required("title", n.Title); err != nil {
		return err
	}

	if n.Author != nil {
		return checkAuthor("author", *n.Author)
	}

	return nil
}

func (n *Snippet) refs() []Ref {
	return append(refsOf(KindHelpCenter, n.HelpCenterRef), authorRefs(n.Author)...)
}

func (n *Snippet) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: n.Title}

	center, ok := s.refs.id(KindHelpCenter, n.HelpCenterRef)
	if !ok {
		def, err := s.q.FindDefaultHelpCenter(ctx, s.ws())
		if err != nil {
			return out, fmt.Errorf("find default help center: %w", err)
		}

		center = def.ID
	}

	return s.writeArticle(ctx, articleWrite{
		center: center, title: n.Title, body: n.Body, visibility: "agent_only", publish: true, author: n.Author,
		created: n.CreatedAt, updated: n.CreatedAt,
	}, out)
}

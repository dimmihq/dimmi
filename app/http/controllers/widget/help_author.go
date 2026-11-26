/*
|--------------------------------------------------------------------------
| Widget Help Author
|--------------------------------------------------------------------------
|
| Who wrote a help article, as the messenger shows it under the title.
|
*/

package widget

import "github.com/dimmihq/dimmi/app/models"

/*
HelpAuthor is who wrote an article: their name and a short-lived
address of their photo, nil without one.
*/
type HelpAuthor struct {
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
authorOf is who wrote an article, nil when no teammate did.
*/
func authorOf(a models.ListPublishedArticlesRow, photo func(path string) *string) *HelpAuthor {
	if a.AuthorName == "" {
		return nil
	}
	out := &HelpAuthor{Name: a.AuthorName}
	if a.AuthorAvatar != nil {
		out.AvatarURL = photo(*a.AuthorAvatar)
	}
	return out
}

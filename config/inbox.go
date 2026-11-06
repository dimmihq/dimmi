/*
|--------------------------------------------------------------------------
| Inbox Settings
|--------------------------------------------------------------------------
|
| How long the shared inbox keeps what members leave behind. A draft
| nobody touched for a month is almost certainly forgotten, so it is
| removed; a draft still being written is never touched.
|
| GIF search is off until the operator gives a GIPHY key. Searches go
| from the browser straight to GIPHY, as its terms require, so the key is
| shown to signed-in teammates and the dashboard's policy only then lets
| the page reach GIPHY.
|
*/

package config

/*
InboxConfig configures the shared inbox.
*/
type InboxConfig struct {
	Drafts DraftsConfig `koanf:"drafts"`
	GIF    GIFConfig    `koanf:"gif"`
}

/*
GIFConfig holds the GIPHY key the composer's GIF search uses; empty
turns GIF search off for every workspace.
*/
type GIFConfig struct {
	GiphyAPIKey string `koanf:"giphy_api_key" secret:"true"`
}

/*
DraftsConfig says how many days an untouched draft is kept before the
daily purge removes it; 0 keeps drafts for ever.
*/
type DraftsConfig struct {
	RetentionDays int `default:"30" koanf:"retention_days"`
}

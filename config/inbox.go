/*
|--------------------------------------------------------------------------
| Inbox Settings
|--------------------------------------------------------------------------
|
| How long the shared inbox keeps what members leave behind. A draft
| nobody touched for a month is almost certainly forgotten, so it is
| removed; a draft still being written is never touched.
|
*/

package config

/*
InboxConfig configures the shared inbox.
*/
type InboxConfig struct {
	Drafts DraftsConfig `koanf:"drafts"`
}

/*
DraftsConfig says how many days an untouched draft is kept before the
daily purge removes it; 0 keeps drafts for ever.
*/
type DraftsConfig struct {
	RetentionDays int `default:"30" koanf:"retention_days"`
}

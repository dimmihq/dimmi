/*
|--------------------------------------------------------------------------
| Help Center Themes
|--------------------------------------------------------------------------
|
| A team gives its help center its own look by uploading a theme as a
| zip. Every upload is checked whole, every template parsed, and only a
| theme with no problems is kept, as a new version that never changes.
| Publishing points the help center at a version, or back at the default
| theme, which starts its readers' pages over; going back to an earlier
| version is publishing it again.
|
| Readers' pages load a version once and keep it parsed: versions never
| change, so the cache needs no other key than the version's id.
|
*/

package helpcenters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/memo"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
MaxThemeArchiveBytes is the largest theme zip an upload takes.
*/
const MaxThemeArchiveBytes = themes.MaxBytes + 1<<20

/*
Failures of managing themes.
*/
var (
	ErrThemeNotFound = fault.New(fault.NotFound, "theme_not_found", "No theme of this help center has this id.")
	ErrThemeInvalid  = fault.New(fault.Invalid, "theme_invalid",
		"The theme has problems; nothing was stored. Fix each one listed in problems and upload again.")
	ErrThemeArchive = fault.New(fault.Invalid, "theme_archive_invalid",
		"Upload the theme as a zip of its folder, holding theme.json at its root.").OnField("file")
	ErrThemeSettingsInvalid = fault.New(fault.Invalid, "theme_settings_invalid",
		"Some values do not fit the theme's settings; see problems.").OnField("settings")
)

/*
Themes keeps help centers' themes.
*/
type Themes struct {
	DB     *database.DB
	Clock  clock.Clock
	loaded *memo.Cache[ids.UUID, *themes.Theme]
	live   *memo.Cache[liveKey, LiveTheme]

	once     sync.Once
	fallback *themes.Theme
	err      error
}

/*
Default returns the default theme, parsed once and kept: the theme
every help center wears until its team publishes its own, and the one
every team's theme falls back to.
*/
func (t *Themes) Default() (*themes.Theme, error) {
	t.once.Do(func() { t.fallback, t.err = themes.LoadDefault() })
	return t.fallback, t.err
}

/*
load checks and parses a team's theme over the default.
*/
func (t *Themes) load(files map[string][]byte) (*themes.Theme, []themes.Problem, error) {
	def, err := t.Default()
	if err != nil {
		return nil, nil, err
	}
	theme, problems := themes.Load(files, def)
	return theme, problems, nil
}

/*
liveKey names a theme version as a help center wears it at one cache
version, which moves on whenever the team changes its settings.
*/
type liveKey struct {
	id      ids.UUID
	version int64
}

/*
NewThemes returns the themes, with a cache of parsed versions.
*/
func NewThemes(db *database.DB, clk clock.Clock) *Themes {
	return &Themes{
		DB: db, Clock: clk,
		loaded: memo.New[ids.UUID, *themes.Theme](memo.Options{Size: 200, TTL: time.Hour}),
		live:   memo.New[liveKey, LiveTheme](memo.Options{Size: 1000, TTL: time.Minute}),
	}
}

/*
ThemeUpload is a theme's zip uploaded for a help center.
*/
type ThemeUpload struct {
	Membership   authz.Membership
	HelpCenterID ids.UUID
	Archive      []byte
}

/*
Upload checks a theme and stores it as the help center's next version.
A theme with problems answers ErrThemeInvalid with every problem under
problems, and nothing is stored.
*/
func (t *Themes) Upload(ctx context.Context, in ThemeUpload) (models.HelpCenterTheme, error) {
	if !in.Membership.Can("help_centers:manage") {
		return models.HelpCenterTheme{}, exceptions.ErrForbidden
	}
	files, err := Unzip(in.Archive)
	if err != nil {
		return models.HelpCenterTheme{}, err
	}
	theme, problems, err := t.load(files)
	if err != nil {
		return models.HelpCenterTheme{}, err
	}
	if len(problems) > 0 {
		return models.HelpCenterTheme{}, ErrThemeInvalid.With("problems", problems)
	}
	var out models.HelpCenterTheme
	err = t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		out, err = t.store(ctx, models.New(tx), in, checked{theme: theme, files: files})
		return err
	})
	return out, err
}

/*
store keeps a checked theme as the help center's next version, with
every file.
*/
func (t *Themes) store(ctx context.Context, q *models.Queries, in ThemeUpload, loaded checked,
) (models.HelpCenterTheme, error) {
	theme, files := loaded.theme, loaded.files
	ws := in.Membership.Workspace.ID
	if _, err := findHelpCenter(ctx, q, ws, in.HelpCenterID); err != nil {
		return models.HelpCenterTheme{}, err
	}
	version, err := q.NextHelpCenterThemeVersion(ctx, models.NextHelpCenterThemeVersionParams{
		WorkspaceID: ws, HelpCenterID: in.HelpCenterID,
	})
	if err != nil {
		return models.HelpCenterTheme{}, fmt.Errorf("next theme version: %w", err)
	}
	manifest, err := json.Marshal(theme.Manifest)
	if err != nil {
		return models.HelpCenterTheme{}, fmt.Errorf("theme manifest: %w", err)
	}
	size := 0
	for _, body := range files {
		size += len(body)
	}
	params := models.CreateHelpCenterThemeParams{
		ID: ids.New(), WorkspaceID: ws, HelpCenterID: in.HelpCenterID, Version: version,
		Name: strings.TrimSpace(theme.Manifest.Name), Manifest: manifest, ContentHash: theme.Hash,
		FileCount: int32(min(len(files), themes.MaxFiles)), SizeBytes: int32(min(size, themes.MaxBytes)),
		Now: t.Clock.Now(),
	}
	if user := in.Membership.Member.UserID; user != (ids.UUID{}) {
		params.CreatedBy = &user
	}
	out, err := q.CreateHelpCenterTheme(ctx, params)
	if err != nil {
		return out, fmt.Errorf("create theme: %w", err)
	}
	for _, name := range sortedNames(files) {
		err := q.CreateHelpCenterThemeFile(ctx, models.CreateHelpCenterThemeFileParams{
			ThemeID: out.ID, WorkspaceID: ws, Path: name, Body: files[name],
		})
		if err != nil {
			return out, fmt.Errorf("store theme file %s: %w", name, err)
		}
	}
	return out, nil
}

/*
ThemeList is a help center's theme versions, newest first, with the
one its pages wear, nil for the default theme.
*/
type ThemeList struct {
	Themes []models.HelpCenterTheme
	Live   *ids.UUID
}

/*
List returns a help center's theme versions.
*/
func (t *Themes) List(ctx context.Context, m authz.Membership, helpCenter ids.UUID) (ThemeList, error) {
	var out ThemeList
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenter)
		if err != nil {
			return err
		}
		out.Live = h.ThemeID
		out.Themes, err = q.ListHelpCenterThemes(ctx, models.ListHelpCenterThemesParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: helpCenter,
		})
		return err
	})
	return out, err
}

/*
Show returns one theme version of a help center.
*/
func (t *Themes) Show(ctx context.Context, m authz.Membership, helpCenter, id ids.UUID,
) (models.HelpCenterTheme, error) {
	var row models.HelpCenterTheme
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = findTheme(ctx, models.New(tx), m.Workspace.ID, themeRef{helpCenter: helpCenter, id: id})
		return err
	})
	return row, err
}

/*
Publish makes a theme version the one the help center's pages wear, or
the default theme when id is nil.
*/
func (t *Themes) Publish(ctx context.Context, m authz.Membership, helpCenter ids.UUID, id *ids.UUID,
) (models.HelpCenter, error) {
	if !m.Can("help_centers:manage") {
		return models.HelpCenter{}, exceptions.ErrForbidden
	}
	var out models.HelpCenter
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenter); err != nil {
			return err
		}
		if id != nil {
			if _, err := findTheme(ctx, q, m.Workspace.ID, themeRef{helpCenter: helpCenter, id: *id}); err != nil {
				return err
			}
		}
		var err error
		out, err = q.SetHelpCenterTheme(ctx, models.SetHelpCenterThemeParams{
			ThemeID: id, Now: t.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: helpCenter,
		})
		return err
	})
	return out, err
}

/*
ThemeSettingsChange is the team's values for the settings of one theme
version of a help center.
*/
type ThemeSettingsChange struct {
	Membership   authz.Membership
	HelpCenterID ids.UUID
	ID           ids.UUID
	Values       map[string]any
}

/*
UpdateSettings stores the team's values for a theme version's settings,
checked against what its theme.json declares, and starts the pages of
the help center wearing it over.
*/
func (t *Themes) UpdateSettings(ctx context.Context, in ThemeSettingsChange) (models.HelpCenterTheme, error) {
	m := in.Membership
	if !m.Can("help_centers:manage") {
		return models.HelpCenterTheme{}, exceptions.ErrForbidden
	}
	var out models.HelpCenterTheme
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := findTheme(ctx, q, m.Workspace.ID, themeRef{helpCenter: in.HelpCenterID, id: in.ID})
		if err != nil {
			return err
		}
		var manifest themes.Manifest
		if err := json.Unmarshal(row.Manifest, &manifest); err != nil {
			return fmt.Errorf("theme manifest: %w", err)
		}
		if problems := manifest.CheckSettings(in.Values); len(problems) > 0 {
			return ErrThemeSettingsInvalid.With("problems", problems)
		}
		raw, err := json.Marshal(in.Values)
		if err != nil {
			return fmt.Errorf("theme settings: %w", err)
		}
		now := t.Clock.Now()
		out, err = q.UpdateHelpCenterThemeSettings(ctx, models.UpdateHelpCenterThemeSettingsParams{
			Settings: raw, Now: now, WorkspaceID: m.Workspace.ID, ID: in.ID,
		})
		if err != nil {
			return fmt.Errorf("update theme settings: %w", err)
		}
		return q.TouchHelpCentersWearingTheme(ctx, models.TouchHelpCentersWearingThemeParams{
			Now: now, WorkspaceID: m.Workspace.ID, ThemeID: &in.ID,
		})
	})
	return out, err
}

/*
themeRef names a theme version of a help center.
*/
type themeRef struct {
	helpCenter ids.UUID
	id         ids.UUID
}

/*
checked is a theme that loaded with no problems, with its files.
*/
type checked struct {
	theme *themes.Theme
	files map[string][]byte
}

/*
findTheme returns a theme version of the help center, ErrThemeNotFound
for one of another help center or none.
*/
func findTheme(ctx context.Context, q *models.Queries, ws ids.UUID, ref themeRef) (models.HelpCenterTheme, error) {
	row, err := q.FindHelpCenterTheme(ctx, models.FindHelpCenterThemeParams{WorkspaceID: ws, ID: ref.id})
	if errors.Is(err, database.ErrNotFound) || (err == nil && row.HelpCenterID != ref.helpCenter) {
		return row, ErrThemeNotFound
	}
	if err != nil {
		return row, fmt.Errorf("find theme: %w", err)
	}
	return row, nil
}

/*
Archive returns a theme version as a zip of its folder, or the default
theme's when id is nil, the starting point of a team's own.
*/
func (t *Themes) Archive(ctx context.Context, m authz.Membership, helpCenter ids.UUID, id *ids.UUID) ([]byte, error) {
	if id == nil {
		files, err := themes.DefaultFiles()
		if err != nil {
			return nil, err
		}
		return Zip(files)
	}
	if _, err := t.Show(ctx, m, helpCenter, *id); err != nil {
		return nil, err
	}
	var files map[string][]byte
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		files, err = t.files(ctx, models.New(tx), m.Workspace.ID, *id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return Zip(files)
}

func (t *Themes) files(ctx context.Context, q *models.Queries, ws, id ids.UUID) (map[string][]byte, error) {
	rows, err := q.ListHelpCenterThemeFiles(ctx, models.ListHelpCenterThemeFilesParams{WorkspaceID: ws, ThemeID: id})
	if err != nil {
		return nil, fmt.Errorf("theme files: %w", err)
	}
	files := make(map[string][]byte, len(rows))
	for _, r := range rows {
		files[r.Path] = r.Body
	}
	return files, nil
}

/*
LiveTheme is a help center's theme as its pages wear it: the parsed
version and the team's values for its settings.
*/
type LiveTheme struct {
	Theme    *themes.Theme
	Settings map[string]any
}

/*
Live returns the theme version a help center's pages wear, parsed once
and kept, with the team's settings as they stood at the help center's
cache version. It reads the version as the help center's workspace.
*/
func (t *Themes) Live(ctx context.Context, workspace, id ids.UUID, version int64) (LiveTheme, error) {
	key := liveKey{id: id, version: version}
	if cached, ok := t.live.Get(key); ok {
		return cached, nil
	}
	out, err := t.read(ctx, workspace, id)
	if err == nil {
		t.live.Set(key, out)
	}
	return out, err
}

func (t *Themes) read(ctx context.Context, workspace, id ids.UUID) (LiveTheme, error) {
	var out LiveTheme
	scoped := tenancy.With(ctx, workspace.String())
	err := t.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := q.FindHelpCenterTheme(ctx, models.FindHelpCenterThemeParams{WorkspaceID: workspace, ID: id})
		if err != nil {
			return fmt.Errorf("find live theme: %w", err)
		}
		if len(row.Settings) > 0 {
			if err := json.Unmarshal(row.Settings, &out.Settings); err != nil {
				return fmt.Errorf("theme settings: %w", err)
			}
		}
		if cached, ok := t.loaded.Get(id); ok {
			out.Theme = cached
			return nil
		}
		files, err := t.files(ctx, q, workspace, id)
		if err != nil {
			return err
		}
		theme, problems, err := t.load(files)
		if err != nil {
			return err
		}
		if len(problems) > 0 {
			return fmt.Errorf("live theme %s no longer loads: %s", id, problems[0])
		}
		t.loaded.Set(id, theme)
		out.Theme = theme
		return nil
	})
	return out, err
}

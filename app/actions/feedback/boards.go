/*
|--------------------------------------------------------------------------
| Boards
|--------------------------------------------------------------------------
|
| A help center's requests are split across boards, such as feature
| requests and bugs. A public board is open to everyone, a private one
| to signed-in customers only, and an internal one to the team alone. A
| board can hold what customers post until the team approves it. Every
| help center has at least one board: the first time anyone looks, a
| help center without one gets its default board, and removing a board
| moves its requests onto another first.
|
*/

package feedback

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Who may see a board on the portal.
*/
const (
	VisibilityPublic   = "public"
	VisibilityPrivate  = "private"
	VisibilityInternal = "internal"
)

/*
Failures of managing boards.
*/
var (
	ErrBoardNotFound = fault.New(fault.NotFound, "feedback_board_not_found",
		"No board of this help center has this id.")
	ErrBoardNameInvalid = fault.New(fault.Invalid, "name_invalid",
		"Board names are 1 to 60 characters.").OnField("name")
	ErrBoardSlugInvalid = fault.New(fault.Invalid, "slug_invalid",
		"Use 1 to 60 lowercase letters, digits and dashes.").OnField("slug")
	ErrBoardSlugTaken = fault.New(fault.Conflict, "slug_taken",
		"Another board of this help center already uses this address.").OnField("slug")
	ErrBoardDescriptionTooLong = fault.New(fault.Invalid, "description_too_long",
		"Descriptions are at most 500 characters.").OnField("description")
	ErrVisibilityInvalid = fault.New(fault.Invalid, "visibility_invalid",
		"The visibility is public, private or internal.").OnField("visibility")
	ErrLastBoard = fault.New(fault.Conflict, "last_board",
		"Keep at least one board.")
	ErrBoardMoveInvalid = fault.New(fault.Invalid, "move_to_invalid",
		"Pick another board of this help center to move the requests to.").OnField("move_to")
	ErrBoardOrderInvalid = fault.New(fault.Invalid, "boards_invalid",
		"List every board of the help center exactly once.").OnField("boards")
)

/*
Board is a board with how many live requests it holds.
*/
type Board struct {
	models.FeedbackBoard
	Posts int32
}

/*
BoardChange adds or changes a board; nil leaves a field as it is. A new
board needs a name and takes its slug from it unless one is given.
*/
type BoardChange struct {
	Name             *string
	Slug             *string
	Description      *string
	Visibility       *string
	ApprovalRequired *bool
}

/*
Boards returns a help center's boards in the team's order, with how
many requests each holds.
*/
func (p *Posts) Boards(ctx context.Context, m authz.Membership, hc ids.UUID) ([]Board, error) {
	if !m.Can("feedback:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []Board
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = counted(ctx, models.New(tx), m.Workspace.ID, hc)
		return err
	})
	return out, err
}

func counted(ctx context.Context, q *models.Queries, ws, hc ids.UUID) ([]Board, error) {
	center, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{WorkspaceID: ws, ID: hc})
	if err != nil {
		return nil, missing(err, helpcenters.ErrHelpCenterNotFound)
	}
	boards, err := boardsOf(ctx, q, center)
	if err != nil {
		return nil, err
	}
	counts, err := q.CountFeedbackByBoard(ctx, models.CountFeedbackByBoardParams{WorkspaceID: ws, HelpCenterID: hc})
	if err != nil {
		return nil, fmt.Errorf("count feature requests by board: %w", err)
	}
	out := make([]Board, len(boards))
	for i, b := range boards {
		out[i] = Board{FeedbackBoard: b}
		for _, c := range counts {
			if c.BoardID != nil && *c.BoardID == b.ID {
				out[i].Posts = c.Posts
			}
		}
	}
	return out, nil
}

/*
boardsOf returns the help center's boards, giving a help center without
any its default board, which takes over its requests.
*/
func boardsOf(ctx context.Context, q *models.Queries, h models.HelpCenter) ([]models.FeedbackBoard, error) {
	boards, err := q.ListFeedbackBoards(ctx, models.ListFeedbackBoardsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil || len(boards) > 0 {
		return boards, err
	}
	first, err := q.CreateFeedbackBoard(ctx, models.CreateFeedbackBoardParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Name: "Feature requests", Slug: "feature-requests",
		Visibility: VisibilityPublic, ApprovalRequired: helpcenters.SettingsOf(h).FeedbackApproval, Now: h.UpdatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create default board: %w", err)
	}
	if err := q.AdoptFeedbackPosts(ctx, models.AdoptFeedbackPostsParams{
		BoardID: &first.ID, WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	}); err != nil {
		return nil, fmt.Errorf("adopt feature requests: %w", err)
	}
	return []models.FeedbackBoard{first}, nil
}

/*
boardOf returns the help center's board with the id, or its first board
when id is nil.
*/
func boardOf(ctx context.Context, q *models.Queries, h models.HelpCenter, id *ids.UUID) (models.FeedbackBoard, error) {
	boards, err := boardsOf(ctx, q, h)
	if err != nil {
		return models.FeedbackBoard{}, err
	}
	if id == nil {
		return boards[0], nil
	}
	for _, b := range boards {
		if b.ID == *id {
			return b, nil
		}
	}
	return models.FeedbackBoard{}, ErrBoardNotFound
}

/*
CreateBoard adds a board after the help center's others.
*/
func (p *Posts) CreateBoard(ctx context.Context, m authz.Membership, hc ids.UUID, in BoardChange) (Board, error) {
	if !m.Can("settings:write") {
		return Board{}, exceptions.ErrForbidden
	}
	b := models.FeedbackBoard{Visibility: VisibilityPublic}
	if err := in.over(&b, true); err != nil {
		return Board{}, err
	}
	ws := m.Workspace.ID
	var out Board
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		center, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{WorkspaceID: ws, ID: hc})
		if err != nil {
			return missing(err, helpcenters.ErrHelpCenterNotFound)
		}
		if _, err := boardsOf(ctx, q, center); err != nil {
			return err
		}
		row, err := q.CreateFeedbackBoard(ctx, models.CreateFeedbackBoardParams{
			WorkspaceID: ws, HelpCenterID: hc, Name: b.Name, Slug: b.Slug, Description: b.Description,
			Visibility: b.Visibility, ApprovalRequired: b.ApprovalRequired, Now: p.Clock.Now(),
		})
		if err != nil {
			return missing(err, ErrBoardSlugTaken)
		}
		out = Board{FeedbackBoard: row}
		return nil
	})
	return out, err
}

/*
UpdateBoard changes a board.
*/
func (p *Posts) UpdateBoard(ctx context.Context, m authz.Membership, id ids.UUID, in BoardChange) (Board, error) {
	if !m.Can("settings:write") {
		return Board{}, exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	var out Board
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		b, err := q.LockFeedbackBoard(ctx, models.LockFeedbackBoardParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return missing(err, ErrBoardNotFound)
		}
		if err := in.over(&b, false); err != nil {
			return err
		}
		if err := slugFree(ctx, q, b); err != nil {
			return err
		}
		row, err := q.UpdateFeedbackBoard(ctx, models.UpdateFeedbackBoardParams{
			Name: b.Name, Slug: b.Slug, Description: b.Description, Visibility: b.Visibility,
			ApprovalRequired: b.ApprovalRequired, Now: p.Clock.Now(), WorkspaceID: ws, ID: id,
		})
		if err != nil {
			return fmt.Errorf("update board: %w", err)
		}
		all, err := counted(ctx, q, ws, row.HelpCenterID)
		if i := slices.IndexFunc(all, func(one Board) bool { return one.ID == row.ID }); i >= 0 {
			out = all[i]
		}
		return err
	})
	return out, err
}

/*
slugFree refuses a slug another board of the help center uses.
*/
func slugFree(ctx context.Context, q *models.Queries, b models.FeedbackBoard) error {
	taken, err := q.FeedbackBoardSlugTaken(ctx, models.FeedbackBoardSlugTakenParams{
		HelpCenterID: b.HelpCenterID, Slug: b.Slug, ID: b.ID,
	})
	if err != nil {
		return fmt.Errorf("check board slug: %w", err)
	}
	if taken {
		return ErrBoardSlugTaken
	}
	return nil
}

/*
DeleteBoard removes a board, moving its requests onto the board moveTo
names, or the help center's first other board.
*/
func (p *Posts) DeleteBoard(ctx context.Context, m authz.Membership, id ids.UUID, moveTo *ids.UUID) error {
	if !m.Can("settings:write") {
		return exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		gone, err := q.LockFeedbackBoard(ctx, models.LockFeedbackBoardParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return missing(err, ErrBoardNotFound)
		}
		boards, err := q.ListFeedbackBoards(ctx, models.ListFeedbackBoardsParams{
			WorkspaceID: ws, HelpCenterID: gone.HelpCenterID,
		})
		if err != nil {
			return fmt.Errorf("list boards: %w", err)
		}
		target, err := boardTarget(slices.DeleteFunc(boards, func(b models.FeedbackBoard) bool { return b.ID == id }),
			moveTo)
		if err != nil {
			return err
		}
		if err := q.MoveFeedbackPostsToBoard(ctx, models.MoveFeedbackPostsToBoardParams{
			TargetID: &target, Now: p.Clock.Now(), WorkspaceID: ws, SourceID: &id,
		}); err != nil {
			return fmt.Errorf("move feature requests to another board: %w", err)
		}
		if err := q.DeleteFeedbackBoard(ctx, models.DeleteFeedbackBoardParams{WorkspaceID: ws, ID: id}); err != nil {
			return fmt.Errorf("delete board: %w", err)
		}
		return nil
	})
}

/*
boardTarget returns the board a removed board's requests move onto:
the one moveTo names among the others, or the first of them.
*/
func boardTarget(others []models.FeedbackBoard, moveTo *ids.UUID) (ids.UUID, error) {
	if len(others) == 0 {
		return ids.UUID{}, ErrLastBoard
	}
	if moveTo == nil {
		return others[0].ID, nil
	}
	if !slices.ContainsFunc(others, func(b models.FeedbackBoard) bool { return b.ID == *moveTo }) {
		return ids.UUID{}, ErrBoardMoveInvalid
	}
	return *moveTo, nil
}

/*
ReorderBoards saves the order of a help center's boards; order names
every board once, first to last.
*/
func (p *Posts) ReorderBoards(ctx context.Context, m authz.Membership, hc ids.UUID, order []ids.UUID,
) ([]Board, error) {
	if !m.Can("settings:write") {
		return nil, exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	var out []Board
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := counted(ctx, q, ws, hc)
		if err != nil {
			return err
		}
		if len(order) != len(current) {
			return ErrBoardOrderInvalid
		}
		for _, b := range current {
			if !slices.Contains(order, b.ID) {
				return ErrBoardOrderInvalid
			}
		}
		if err := q.SaveFeedbackBoardPositions(ctx, models.SaveFeedbackBoardPositionsParams{
			Now: p.Clock.Now(), BoardIds: order, WorkspaceID: ws, HelpCenterID: hc,
		}); err != nil {
			return fmt.Errorf("save board order: %w", err)
		}
		out, err = counted(ctx, q, ws, hc)
		return err
	})
	return out, err
}

/*
over writes the change over a board, checking each field it names. A
new board takes its slug from its name unless one is given.
*/
func (c BoardChange) over(b *models.FeedbackBoard, fresh bool) error {
	if err := c.naming(b, fresh); err != nil {
		return err
	}
	if c.Description != nil {
		d := strings.TrimSpace(*c.Description)
		if len([]rune(d)) > 500 {
			return ErrBoardDescriptionTooLong
		}
		b.Description = d
	}
	if c.Visibility != nil {
		if !slices.Contains([]string{VisibilityPublic, VisibilityPrivate, VisibilityInternal}, *c.Visibility) {
			return ErrVisibilityInvalid
		}
		b.Visibility = *c.Visibility
	}
	if c.ApprovalRequired != nil {
		b.ApprovalRequired = *c.ApprovalRequired
	}
	return nil
}

/*
naming writes the change's name and slug over a board.
*/
func (c BoardChange) naming(b *models.FeedbackBoard, fresh bool) error {
	if c.Name != nil {
		name := strings.TrimSpace(*c.Name)
		if name == "" || len([]rune(name)) > 60 {
			return ErrBoardNameInvalid
		}
		b.Name = name
	}
	if b.Name == "" {
		return ErrBoardNameInvalid
	}
	switch {
	case c.Slug != nil:
		if !boardSlug().MatchString(*c.Slug) {
			return ErrBoardSlugInvalid
		}
		b.Slug = *c.Slug
	case fresh:
		b.Slug = cmp.Or(helpcenters.Slugify(b.Name, 60), "board")
	}
	return nil
}

func boardSlug() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9-]{1,60}$`)
}

/*
readable says whether a reader sees the board on the portal: anyone
sees a public board, signed-in readers a private one, and nobody an
internal one.
*/
func readable(b models.FeedbackBoard, signedIn bool) bool {
	return b.Visibility == VisibilityPublic || (b.Visibility == VisibilityPrivate && signedIn)
}

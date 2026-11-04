/*
|--------------------------------------------------------------------------
| Member Import Controller
|--------------------------------------------------------------------------
|
| Admins upload a spreadsheet of members to change many at once, or to
| see first what it would change. The answer is the report of every row,
| with each refusal in the reader's language.
|
*/

package v1

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/actions/memberimports"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
MemberImportOutput is the report of one import.
*/
type MemberImportOutput struct {
	Body fhttp.Resource[resources.MemberImport]
}

/*
MemberImportController runs member imports in the workspace in play.
*/
type MemberImportController struct {
	Import *memberimports.ImportMembers
}

/*
Store runs the uploaded file, or with dry_run only checks it.
*/
func (c *MemberImportController) Store(
	ctx context.Context, in *requests.StoreMemberImportRequest,
) (*MemberImportOutput, error) {
	defer func() { _ = in.RawBody.RemoveAll() }()

	uploads := in.RawBody.File["file"]
	if len(uploads) == 0 {
		return nil, exceptions.ErrImportUnreadable
	}

	file, err := uploads[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded import: %w", err)
	}

	defer func() { _ = file.Close() }()

	report, err := c.Import.Handle(ctx, memberimports.ImportMembersInput{
		Principal: caller(ctx), Membership: membership(ctx), File: file, DryRun: in.DryRun(),
	})
	if err != nil {
		return nil, err
	}

	shaped := resources.NewMemberImport(report, func(err error) resources.InvitationError {
		code := fault.CodeOf(err)

		return resources.InvitationError{Code: code, Message: i18n.T(ctx, "errors."+code, nil)}
	})

	return &MemberImportOutput{Body: fhttp.NewResource(shaped)}, nil
}

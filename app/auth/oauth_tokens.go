/*
|--------------------------------------------------------------------------
| OAuth Access Tokens
|--------------------------------------------------------------------------
|
| An app a person connected through OAuth, such as an AI agent, acts as
| that person in the one workspace they chose, with the access they
| approved. It is treated like a personal access token in every way that
| matters: the same kind of caller, bound to one workspace, holding only
| the abilities it was granted, and never more than the person's role
| allows. A grant whose person left the workspace, or whose account was
| deleted, stops working at once.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OAuthTokenPrefix starts every secret the OAuth server issues: client
secrets, codes and tokens, as dmi_oat_ for access tokens.
*/
const OAuthTokenPrefix = "dmi_"

/*
AuthorizationPath is the page a person approves an app's request on.
*/
const AuthorizationPath = "/oauth/authorize"

/*
OfflineAccess is the scope clients ask for to get a refresh token. It is
accepted and grants nothing more.
*/
const OfflineAccess = "offline_access"

/*
OAuthScopes are the scopes an app may ask for: every ability group but
the operator's own, plus offline access.
*/
func OAuthScopes() []string {
	var out []string

	for _, group := range AbilityGroups() {
		if !strings.HasPrefix(group, "system:") {
			out = append(out, group)
		}
	}

	return append(out, OfflineAccess)
}

/*
GrantedScopes widens approved scopes by their hierarchy: writing to
something implies reading it.
*/
func GrantedScopes(scopes []string) []string {
	out := slices.Clone(scopes)

	for _, scope := range scopes {
		if area, ok := strings.CutSuffix(scope, ":write"); ok && !slices.Contains(out, area+":read") {
			out = append(out, area+":read")
		}
	}

	slices.Sort(out)

	return out
}

/*
OAuthGuard resolves OAuth access tokens into the member they act as.
Server is the resolver of the OAuth server's access tokens.
*/
type OAuthGuard struct {
	DB     *database.DB
	Server fauth.TokenResolver
}

var _ fauth.TokenResolver = (*OAuthGuard)(nil)

/*
Resolve returns the member an OAuth access token acts as.
*/
func (g *OAuthGuard) Resolve(r *fhttp.Request, plain string) (fauth.Principal, error) {
	p, err := g.Server.Resolve(r, plain)
	if err != nil {
		return nil, err
	}

	if p.Tenant() == "" {
		return nil, fauth.ErrInvalidToken
	}

	workspaceID, err := ids.Parse(p.Tenant())
	if err != nil {
		return nil, fauth.ErrInvalidToken
	}

	member, err := grantMember(r.Context(), g.DB, workspaceID, p.ID())
	if err != nil {
		return nil, err
	}

	user, err := liveUser(r.Context(), g.DB, p.ID())
	if errors.Is(err, fauth.ErrNoCredentials) {
		return nil, fauth.ErrInvalidToken
	}

	if err != nil {
		return nil, err
	}

	credential := ids.Nil()
	if identity, ok := p.(*fauth.Identity); ok {
		credential = identity.Credential
	}

	return &Principal{
		User: user, Kind: KindToken, CredentialID: credential, WorkspaceID: workspaceID, MemberID: member.ID,
		Granted: GrantedScopes(p.Abilities()), IP: ClientAddr(r), Connected: true,
	}, nil
}

/*
grantMember returns the active member a grant's person is in its
workspace, or fauth.ErrInvalidToken when they are not one any more.
*/
func grantMember(ctx context.Context, db *database.DB, workspaceID, userID ids.UUID) (models.WorkspaceMember, error) {
	var member models.WorkspaceMember

	err := db.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		member, err = models.New(tx).FindWorkspaceMemberForUser(ctx, models.FindWorkspaceMemberForUserParams{
			WorkspaceID: workspaceID, UserID: userID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return member, fauth.ErrInvalidToken
	}

	if err != nil {
		return member, fmt.Errorf("load grant member: %w", err)
	}

	if member.Status != string(enums.MemberStatusActive) {
		return member, fauth.ErrInvalidToken
	}

	return member, nil
}

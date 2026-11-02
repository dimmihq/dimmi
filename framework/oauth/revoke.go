/*
|--------------------------------------------------------------------------
| Token Revocation
|--------------------------------------------------------------------------
|
| A client signing a user out revokes its tokens here. Revoking an access
| token ends that token alone; revoking a refresh token ends the whole
| grant, since a client giving up its refresh token is done with the
| user. The answer is always 200, whether the token was known, already
| revoked or belongs to another client, so the endpoint cannot be used to
| test whether a token exists; another client's token is never touched.
|
*/

package oauth

import (
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/http"
)

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if err := s.revoke(w, r); err != nil {
		s.writeError(w, r, err)

		return
	}

	noStore(w)
	w.WriteHeader(http.StatusOK)
}

/*
revoke authenticates the client and revokes the token if it is one of
the client's.
*/
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) error {
	if err := parseForm(w, r); err != nil {
		return err
	}

	client, err := s.authenticate(r)
	if err != nil {
		return err
	}

	plain := r.PostForm.Get("token")
	if plain == "" {
		return refuse(CodeInvalidRequest, "token is required.")
	}

	if !s.wellFormed(plain, kindAccess) && !s.wellFormed(plain, kindRefresh) {
		return nil
	}

	tok, err := findToken(r.Context(), s.db, digest(plain))
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find token to revoke: %w", err)
	}

	if tok.grant.clientID != client.ID {
		return nil
	}

	if tok.kind == tokenRefresh {
		return s.RevokeGrant(r.Context(), tok.grant.id)
	}

	return revokeToken(r.Context(), s.db, tok.id, s.clock.Now())
}

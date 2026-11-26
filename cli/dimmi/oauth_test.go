package dimmi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFlowSendsPKCEAndChecksState(t *testing.T) {
	var verifier string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		verifier = r.PostForm.Get("code_verifier")
		answer := map[string]any{"access_token": "at", "refresh_token": "rt", "expires_in": 3600}
		_ = json.NewEncoder(w).Encode(answer)
	}))
	defer srv.Close()

	meta := Metadata{
		Issuer: srv.URL, AuthorizationEndpoint: srv.URL + "/oauth/authorize", TokenEndpoint: srv.URL + "/oauth/token",
	}

	f, err := NewFlow(meta, ClientID, "http://127.0.0.1:5555/callback")
	if err != nil {
		t.Fatal(err)
	}

	u, _ := url.Parse(f.AuthorizeURL())
	q := u.Query()

	if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != ClientID ||
		q.Get("redirect_uri") != f.RedirectURI {
		t.Fatalf("authorize query %v", q)
	}

	wrong := url.Values{"code": {"c"}, "state": {"wrong"}}
	if _, err := f.Exchange(context.Background(), srv.Client(), srv.URL, wrong); err == nil {
		t.Fatal("accepted another request's state")
	}

	answer := url.Values{"code": {"c"}, "state": {q.Get("state")}, "iss": {srv.URL}}

	tok, err := f.Exchange(context.Background(), srv.Client(), srv.URL, answer)
	if err != nil || tok.AccessToken != "at" || tok.RefreshToken != "rt" {
		t.Fatalf("%+v %v", tok, err)
	}

	sum := sha256.Sum256([]byte(verifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != q.Get("code_challenge") {
		t.Fatal("the verifier sent doesn't match the challenge")
	}
}

package dimmi_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"

	dimmi "github.com/dimmihq/dimmi-go"
	"github.com/dimmihq/dimmi-go/api"
)

func newClient(t *testing.T, h http.HandlerFunc, opts ...dimmi.Option) *dimmi.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := dimmi.New(append([]dimmi.Option{
		dimmi.WithToken("dmi_test"), dimmi.WithWorkspace("ws_1"), dimmi.WithBaseURL(srv.URL),
	}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHeadersAndGeneratedCall(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/conversations" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer dmi_test" || r.Header.Get("Dimmi-Workspace") != "ws_1" {
			t.Errorf("headers %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"conv_1"}],"meta":{"next_cursor":null}}`)
	})
	res, err := c.API.ConversationsIndexWithResponse(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.JSON200 == nil || len(*res.JSON200.Data) != 1 {
		t.Fatalf("body %s", res.Body)
	}
}

func TestRetriesHonourRetryAfterAndKeepIdempotencyKey(t *testing.T) {
	var calls int32
	var keys []string
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"email":"a@b.test"}` {
			t.Errorf("body %q", body)
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			_, _ = io.WriteString(w, `{"code":"rate_limited","message":"slow down"}`)
			return
		}
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"data":{"id":"ct_1"}}`)
	})
	var out struct{ Data struct{ ID string } }
	if err := c.Do(context.Background(), "POST", "/contacts", nil, map[string]string{"email": "a@b.test"}, &out); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || out.Data.ID != "ct_1" {
		t.Fatalf("calls %d out %+v", calls, out)
	}
	if keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("keys %v", keys)
	}
}

func TestExplicitIdempotencyKey(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") != "order-42" {
			t.Errorf("key %q", r.Header.Get("Idempotency-Key"))
		}
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{}`)
	})
	ctx := dimmi.WithIdempotencyKey(context.Background(), "order-42")
	if err := c.Do(ctx, "POST", "/contacts", nil, map[string]string{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.API.ContactsStoreWithResponse(context.Background(), nil, api.ContactsStoreJSONRequestBody{}, dimmi.IdempotencyKey("order-42")); err != nil {
		t.Fatal(err)
	}
}

func TestTypedErrors(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = io.WriteString(w, `{"code":"validation_failed","message":"Check the fields.","errors":{"email":["is invalid"]}}`)
	})
	_, err := c.API.ContactsStoreWithResponse(context.Background(), nil, api.ContactsStoreJSONRequestBody{})
	var apiErr *dimmi.Error
	if !errors.As(err, &apiErr) || !errors.Is(err, dimmi.ErrValidation) {
		t.Fatalf("err %v", err)
	}
	if apiErr.Code != "validation_failed" || apiErr.Fields["email"][0] != "is invalid" {
		t.Fatalf("%+v", apiErr)
	}
	err = c.Do(context.Background(), "GET", "/contacts/x", nil, nil, nil)
	if !errors.Is(err, dimmi.ErrValidation) || errors.Is(err, dimmi.ErrNotFound) {
		t.Fatalf("err %v", err)
	}
}

func TestNoRetryOnClientErrorAndGiveUpAfterMax(t *testing.T) {
	var calls int32
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(503)
	}, dimmi.WithMaxRetries(2))
	err := c.Do(context.Background(), "GET", "/contacts", nil, nil, nil)
	if !errors.Is(err, dimmi.ErrServer) || calls != 3 {
		t.Fatalf("err %v calls %d", err, calls)
	}
}

func TestCursorPagination(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "2" {
			t.Errorf("limit lost: %s", r.URL.RawQuery)
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = io.WriteString(w, `{"data":[{"id":"a"},{"id":"b"}],"meta":{"next_cursor":"c2"}}`)
		case "c2":
			_, _ = io.WriteString(w, `{"data":[{"id":"c"}],"meta":{"next_cursor":null}}`)
		}
	})
	items, err := dimmi.List[struct{ ID string }](c, "/contacts", url.Values{"limit": {"2"}}).All(context.Background())
	if err != nil || len(items) != 3 || items[2].ID != "c" {
		t.Fatalf("%v %v", items, err)
	}
}

func TestPagePagination(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = io.WriteString(w, `{"data":[{"id":"a"},{"id":"b"}]}`)
		case "2":
			_, _ = io.WriteString(w, `{"data":[{"id":"c"}]}`)
		default:
			t.Errorf("page %s", r.URL.Query().Get("page"))
		}
	})
	items, err := dimmi.Pages[struct{ ID string }](c, "/audiences", url.Values{"limit": {"2"}}).All(context.Background())
	if err != nil || len(items) != 3 {
		t.Fatalf("%v %v", items, err)
	}
}

func TestClaims(t *testing.T) {
	claims, err := dimmi.Claims(dimmi.User{
		ID: "42", Email: "ada@example.test", Name: "Ada", Phone: "+14155550100",
		SignedUpAt: time.Unix(1700000000, 0),
		Company:    &dimmi.Company{ID: "c1", Name: "Acme", Attributes: map[string]any{"plan": "pro"}},
		Attributes: map[string]any{"seats": 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := claims["email_verified"]; ok {
		t.Fatal("email must not be verified unless said")
	}
	attrs := claims["custom_attributes"].(map[string]any)
	if attrs["signed_up_at"] != int64(1700000000) || attrs["seats"] != 3 {
		t.Fatalf("%v", attrs)
	}
	if claims["company"].(map[string]any)["custom_attributes"].(map[string]any)["plan"] != "pro" {
		t.Fatalf("%v", claims["company"])
	}
	for _, c := range []struct {
		user dimmi.User
		code string
	}{
		{dimmi.User{}, "identify_user_invalid"},
		{dimmi.User{ID: "1", Attributes: map[string]any{"Plan": 1}}, "attribute_key_invalid"},
		{dimmi.User{ID: "1", Attributes: map[string]any{"plan": []int{1}}}, "attribute_value_invalid"},
		{dimmi.User{ID: "1", Company: &dimmi.Company{}}, "identify_user_invalid"},
	} {
		var ie *dimmi.IdentityError
		if _, err := dimmi.Claims(c.user); !errors.As(err, &ie) || ie.Code != c.code {
			t.Errorf("want %s got %v", c.code, err)
		}
	}
}

func TestSignHS256(t *testing.T) {
	secret := "dmi_idsec_abc"
	token, err := dimmi.SignIdentityToken(dimmi.User{ID: "7", Email: "a@b.test", EmailVerified: true},
		dimmi.IdentityOptions{Secret: secret, KeyID: "isk_1", Audience: "dimmi", Nonce: "n1"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jwt.Parse(token, func(tk *jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		t.Fatal(err)
	}
	c := parsed.Claims.(jwt.MapClaims)
	if parsed.Header["kid"] != "isk_1" || c["sub"] != "7" || c["email_verified"] != true || c["aud"] != "dimmi" || c["nonce"] != "n1" {
		t.Fatalf("%v %v", parsed.Header, c)
	}
	if c["exp"].(float64)-c["iat"].(float64) != 600 {
		t.Fatalf("lifetime %v", c)
	}
}

func TestSignErrors(t *testing.T) {
	t.Setenv("DIMMI_IDENTITY_SECRET", "")
	t.Setenv("DIMMI_IDENTITY_PRIVATE_KEY", "")
	t.Setenv("DIMMI_IDENTITY_KID", "")
	cases := map[string]dimmi.IdentityOptions{
		"secret_missing":     {},
		"expires_in_invalid": {Secret: "s", ExpiresIn: 2 * time.Hour},
		"kid_required":       {PrivateKey: "-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----"},
	}
	for code, opts := range cases {
		var ie *dimmi.IdentityError
		if _, err := dimmi.SignIdentityToken(dimmi.User{ID: "1"}, opts); !errors.As(err, &ie) || ie.Code != code {
			t.Errorf("want %s got %v", code, err)
		}
	}
}

func pkcs8(t *testing.T, key any) string {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestSignES256AndEdDSA(t *testing.T) {
	ec, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	token, err := dimmi.SignIdentityToken(dimmi.User{ID: "9"}, dimmi.IdentityOptions{PrivateKey: pkcs8(t, ec), KeyID: "isk_es"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return &ec.PublicKey, nil }, jwt.WithValidMethods([]string{"ES256"}))
	if err != nil || parsed.Header["kid"] != "isk_es" {
		t.Fatalf("%v %v", err, parsed)
	}
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	token, err = dimmi.SignIdentityToken(dimmi.User{ID: "9"}, dimmi.IdentityOptions{PrivateKey: pkcs8(t, priv), KeyID: "isk_ed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return pub, nil }, jwt.WithValidMethods([]string{"EdDSA"})); err != nil {
		t.Fatal(err)
	}
}

func TestSnippetEscapes(t *testing.T) {
	html := dimmi.MessengerSnippet(dimmi.SnippetOptions{AppID: `wgt_1"`, Host: "https://d.test/", Token: "a</script>b", Nonce: "n"})
	if strings.Contains(html, "a</script>b") || !strings.Contains(html, `data-app-id="wgt_1&#34;"`) ||
		!strings.Contains(html, `src="https://d.test/widget/v1/loader.js"`) || !strings.Contains(html, `nonce="n"`) {
		t.Fatal(html)
	}
}

func TestTokenHandler(t *testing.T) {
	h := dimmi.TokenHandler(func(r *http.Request) (*dimmi.User, error) {
		if r.URL.Query().Get("anon") != "" {
			return nil, nil
		}
		return &dimmi.User{ID: "1"}, nil
	}, dimmi.IdentityOptions{Secret: "s"})
	for _, path := range []string{"/", "/?anon=1"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		var body map[string]*string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Header().Get("Cache-Control") != "no-store, private" || (path == "/") != (body["token"] != nil) {
			t.Fatalf("%s: %v %s", path, rec.Header(), rec.Body)
		}
	}
}

func TestWebhookVerify(t *testing.T) {
	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	signer, _ := standardwebhooks.NewWebhook(secret)
	body := []byte(`{"id":"evt_1","object":"event","type":"conversation.created","created_at":"2026-11-12T10:00:00Z","workspace_id":"ws_1","actor":{"type":"contact","id":"ct_1"},"data":{"object":{"id":"conv_1"}}}`)
	now := time.Now()
	sig, _ := signer.Sign("evt_1", now, body)
	headers := http.Header{}
	headers.Set("webhook-id", "evt_1")
	headers.Set("webhook-timestamp", strings.TrimSpace(jsonInt(now.Unix())))
	headers.Set("webhook-signature", sig)

	wh, err := dimmi.NewWebhook(secret)
	if err != nil {
		t.Fatal(err)
	}
	event, err := wh.Verify(body, headers)
	if err != nil {
		t.Fatal(err)
	}
	var conv struct{ ID string }
	if event.Type != "conversation.created" || event.DecodeObject(&conv) != nil || conv.ID != "conv_1" || *event.Actor.ID != "ct_1" {
		t.Fatalf("%+v", event)
	}
	if _, err := wh.Verify(append(body, ' '), headers); !errors.Is(err, dimmi.ErrWebhookVerification) {
		t.Fatalf("tampered body passed: %v", err)
	}

	var got string
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(string(body)))
	req.Header = headers
	wh.Handler(func(_ *http.Request, e *dimmi.Event) error { got = e.ID; return nil }).ServeHTTP(rec, req)
	if rec.Code != 204 || got != "evt_1" {
		t.Fatalf("%d %s", rec.Code, got)
	}
}

func jsonInt(n int64) string { b, _ := json.Marshal(n); return string(b) }

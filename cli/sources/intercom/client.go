/*
Package intercom reads an Intercom workspace with the customer's own
access token, through Intercom's REST API version 2.18, and translates
what it reads into dimmi's import format.

Everything is read; nothing is written to Intercom. The token comes from
a private app the customer creates in their own Developer Hub, and never
leaves their machine.
*/
package intercom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

/*
APIVersion is the Intercom API version every request asks for.
*/
const APIVersion = "2.18"

/*
regionBase returns the API address of a region where Intercom hosts
workspaces.
*/
func regionBase(region string) (string, bool) {
	switch strings.ToLower(region) {
	case "us":
		return "https://api.intercom.io", true
	case "eu":
		return "https://api.eu.intercom.io", true
	case "au":
		return "https://api.au.intercom.io", true
	}

	return "", false
}

/*
APIError is an error answer from Intercom.
*/
type APIError struct {
	Status int
	Code   string
	Detail string
	Path   string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("Intercom answered %d %s on %s: %s", e.Status, e.Code, e.Path, e.Detail)
	}

	return fmt.Sprintf("Intercom answered %d on %s", e.Status, e.Path)
}

/*
Forbidden reports whether the token lacks the permission for the call.
*/
func (e *APIError) Forbidden() bool {
	return e.Status == http.StatusForbidden || e.Status == http.StatusUnauthorized && e.Code == "token_unauthorized"
}

/*
Client calls Intercom's API with a token. HTTP should carry the rate
limiter; Base is the region's address.
*/
type Client struct {
	HTTP  *http.Client
	Base  string
	Token string
}

/*
NewClient returns a client for the region, us, eu or au.
*/
func NewClient(hc *http.Client, region, token string) (*Client, error) {
	base, ok := regionBase(region)
	if !ok {
		return nil, fmt.Errorf("unknown Intercom region %q: use us, eu or au", region)
	}

	if hc == nil {
		hc = http.DefaultClient
	}

	return &Client{HTTP: hc, Base: base, Token: token}, nil
}

/*
request is one call to Intercom's API: a method, a path, its query and
a body sent as JSON.
*/
type request struct {
	method string
	path   string
	query  url.Values
	body   any
}

/*
do sends a request and decodes the JSON answer.
*/
func (c *Client) do(ctx context.Context, r request, out any) error {
	u := c.Base + r.path
	if len(r.query) > 0 {
		u += "?" + r.query.Encode()
	}

	var reader io.Reader

	if r.body != nil {
		raw, err := json.Marshal(r.body)
		if err != nil {
			return err
		}

		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, r.method, u, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Intercom-Version", APIVersion)
	req.Header.Set("Accept", "application/json")

	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return err
	}

	return decode(res.StatusCode, r.path, raw, out)
}

/*
decode reads Intercom's answer: an error for a failed call, otherwise
the JSON into out.
*/
func decode(statusCode int, path string, raw []byte, out any) error {
	if statusCode >= 300 {
		e := &APIError{Status: statusCode, Path: path}

		var list struct {
			Errors []struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"errors"`
		}

		if json.Unmarshal(raw, &list) == nil && len(list.Errors) > 0 {
			e.Code, e.Detail = list.Errors[0].Code, list.Errors[0].Message
		}

		return e
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("read Intercom's answer on %s: %w", path, err)
	}

	return nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, request{method: http.MethodGet, path: path, query: query}, out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, request{method: http.MethodPost, path: path, body: body}, out)
}

/*
Open downloads a file. Intercom serves attachments from its CDN with
signed links, which take no token; the token is sent only to Intercom's
own API hosts.
*/
func (c *Client) Open(ctx context.Context, link string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, http.NoBody)
	if err != nil {
		return nil, err
	}

	if u, err := url.Parse(link); err == nil && strings.HasPrefix(c.Base, u.Scheme+"://"+u.Host) {
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Intercom-Version", APIVersion)
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()

		return nil, fmt.Errorf("download answered %d", res.StatusCode)
	}

	return res.Body, nil
}

/*
isForbidden reports whether err is Intercom refusing the token a
permission.
*/
func isForbidden(err error) bool {
	var e *APIError

	return errors.As(err, &e) && e.Forbidden()
}

/*
isNotFound reports whether err is a 404.
*/
func isNotFound(err error) bool {
	var e *APIError

	return errors.As(err, &e) && e.Status == http.StatusNotFound
}

/*
|--------------------------------------------------------------------------
| HTTP Types
|--------------------------------------------------------------------------
|
| Application code speaks HTTP through this package alone, so every route,
| middleware and guard uses the same names and the standard library stays
| an implementation detail. The names are aliases, not new types, so any
| standard handler or middleware fits without conversion.
|
*/

package http

import (
	"io"
	"mime/multipart"
	"net/http"
)

/*
Request is an incoming HTTP request.
*/
type Request = http.Request

/*
ResponseWriter writes the response to a request.
*/
type ResponseWriter = http.ResponseWriter

/*
Handler answers a request.
*/
type Handler = http.Handler

/*
HandlerFunc adapts a function to a Handler.
*/
type HandlerFunc = http.HandlerFunc

/*
Header is a set of request or response headers.
*/
type Header = http.Header

/*
Cookie is a cookie sent or received.
*/
type Cookie = http.Cookie

/*
MultipartForm is a parsed multipart upload, as the RawBody of a route's
input.
*/
type MultipartForm = multipart.Form

/*
SameSite is the cross-site policy of a cookie.
*/
type SameSite = http.SameSite

/*
The cross-site policies a cookie can carry. Lax, the usual choice, sends
the cookie on top-level navigation from other sites but not on their
background requests.
*/
const (
	SameSiteLaxMode    = http.SameSiteLaxMode
	SameSiteStrictMode = http.SameSiteStrictMode
	SameSiteNoneMode   = http.SameSiteNoneMode
)

/*
The request methods routes are registered with.
*/
const (
	MethodGet     = http.MethodGet
	MethodHead    = http.MethodHead
	MethodPost    = http.MethodPost
	MethodPut     = http.MethodPut
	MethodPatch   = http.MethodPatch
	MethodDelete  = http.MethodDelete
	MethodOptions = http.MethodOptions
)

/*
The response statuses handlers answer with.
*/
const (
	StatusOK                    = http.StatusOK
	StatusCreated               = http.StatusCreated
	StatusAccepted              = http.StatusAccepted
	StatusNoContent             = http.StatusNoContent
	StatusMovedPermanently      = http.StatusMovedPermanently
	StatusFound                 = http.StatusFound
	StatusSeeOther              = http.StatusSeeOther
	StatusNotModified           = http.StatusNotModified
	StatusTemporaryRedirect     = http.StatusTemporaryRedirect
	StatusBadRequest            = http.StatusBadRequest
	StatusUnauthorized          = http.StatusUnauthorized
	StatusForbidden             = http.StatusForbidden
	StatusNotFound              = http.StatusNotFound
	StatusMethodNotAllowed      = http.StatusMethodNotAllowed
	StatusConflict              = http.StatusConflict
	StatusGone                  = http.StatusGone
	StatusLocked                = http.StatusLocked
	StatusRequestEntityTooLarge = http.StatusRequestEntityTooLarge
	StatusUnprocessableEntity   = http.StatusUnprocessableEntity
	StatusUpgradeRequired       = http.StatusUpgradeRequired
	StatusPaymentRequired       = http.StatusPaymentRequired
	StatusRequestTimeout        = http.StatusRequestTimeout
	StatusBadGateway            = http.StatusBadGateway
	StatusGatewayTimeout        = http.StatusGatewayTimeout
	StatusTooManyRequests       = http.StatusTooManyRequests
	StatusInternalServerError   = http.StatusInternalServerError
	StatusServiceUnavailable    = http.StatusServiceUnavailable
)

/*
MaxBytesReader limits how much of a request body a handler reads, answering
an error once the limit is passed.
*/
func MaxBytesReader(w ResponseWriter, body io.ReadCloser, limit int64) io.ReadCloser {
	return http.MaxBytesReader(w, body, limit)
}

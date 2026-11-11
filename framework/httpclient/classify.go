/*
|--------------------------------------------------------------------------
| Failure Classes
|--------------------------------------------------------------------------
|
| A request that got no answer failed somewhere on the way: the name did
| not resolve, nothing listened, TLS refused the certificate, the time
| ran out, or the destination was not allowed. Callers that record or
| report failures care which, without depending on network packages, so
| the client names the class of every error it returns.
|
*/

package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"syscall"
)

/*
The classes Classify names a failed request by. ClassNetwork is any
other failure on the way, such as a connection reset.
*/
const (
	ClassTimeout          = "timeout"
	ClassDNS              = "dns_failed"
	ClassRefused          = "connect_refused"
	ClassTLS              = "tls_failed"
	ClassBlocked          = "ssrf_blocked"
	ClassTooLarge         = "response_too_large"
	ClassTooManyRedirects = "redirect_not_followed"
	ClassNetwork          = "network"
)

/*
Classify names the class of an error a request returned, empty for no
error. A destination refused by the connection checks counts as blocked
even when the refusal came back wrapped in another error.
*/
func Classify(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrBlockedHost):
		return ClassBlocked
	case errors.Is(err, ErrTooLarge):
		return ClassTooLarge
	case errors.Is(err, ErrTooManyRedirects):
		return ClassTooManyRedirects
	case isTimeout(err):
		return ClassTimeout
	}

	return classifyConnection(err)
}

/*
isTimeout reports a request that ran out of time, at any step.
*/
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	netErr, ok := errors.AsType[net.Error](err)

	return ok && netErr.Timeout()
}

/*
classifyConnection names failures to resolve, connect or shake hands.
*/
func classifyConnection(err error) string {
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return ClassDNS
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return ClassRefused
	}

	if isTLS(err) {
		return ClassTLS
	}

	return ClassNetwork
}

/*
isTLS reports a failed TLS handshake or a certificate that did not
verify.
*/
func isTLS(err error) bool {
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return true
	}

	if _, ok := errors.AsType[tls.RecordHeaderError](err); ok {
		return true
	}

	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return true
	}

	_, ok := errors.AsType[x509.HostnameError](err)

	return ok
}

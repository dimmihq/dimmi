/*
|--------------------------------------------------------------------------
| How an Address Sends
|--------------------------------------------------------------------------
|
| Every address says exactly what a customer sees as its From now: its
| own address once its domain is verified, its forwarding address on our
| domain otherwise, and why it is held back when its domain lost its
| records or sending is paused.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/email"

/*
WithSending fills in how the address sends from the sender worked out
for it.
*/
func (a EmailAddress) WithSending(s email.Sender) EmailAddress {
	a.SendingState, a.SendsAs = s.State, `"`+a.DisplayName+`" <`+s.Email+`>`
	a.SendingBlockedReason = nil

	if s.BlockedReason != "" {
		reason := s.BlockedReason
		a.SendingBlockedReason = &reason
	}

	a.SendingIdentity.From = a.SendsAs
	a.SendingIdentity.UsesDefaultSender = s.State != email.SendingOwnDomain

	switch {
	case s.State == email.SendingOwnDomain:
		a.SendingIdentity.DomainStatus = "verified"
	case s.BlockedReason == email.BlockedDomainDegraded:
		a.SendingIdentity.DomainStatus = "degraded"
	}

	return a
}

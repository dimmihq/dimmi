package emaildns_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/actions/email"
)

func TestSendingStates(t *testing.T) {
	t.Parallel()

	states := email.SendingStates{Domains: map[string]string{
		"acme.test": email.DomainVerified, "old.test": email.DomainDegraded, "new.test": email.DomainVerifying,
	}}
	forwarding := "acme@inbound.test"

	cases := []struct {
		name, address string
		paused        bool
		want          email.Sender
	}{
		{
			"sends from a verified domain", "Support@Acme.test", false,
			email.Sender{Email: "Support@Acme.test", State: email.SendingOwnDomain},
		},
		{
			"falls back while a domain is degraded", "help@old.test", false,
			email.Sender{
				Email: forwarding, State: email.SendingDefaultSender, BlockedReason: email.BlockedDomainDegraded,
			},
		},
		{
			"falls back until a domain is verified", "help@new.test", false,
			email.Sender{Email: forwarding, State: email.SendingDefaultSender},
		},
		{
			"falls back for a domain nobody added", "help@else.test", false,
			email.Sender{Email: forwarding, State: email.SendingDefaultSender},
		},
		{
			"sends the forwarding address as itself", forwarding, false,
			email.Sender{Email: forwarding, State: email.SendingDefaultSender},
		},
		{
			"says sending is paused before anything else", "help@old.test", true,
			email.Sender{
				Email: forwarding, State: email.SendingDefaultSender, BlockedReason: email.BlockedSendingPaused,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := states
			s.Paused = c.paused

			assert.Equal(t, c.want, s.For(c.address, forwarding))
		})
	}
}

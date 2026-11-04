package emaildns_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/lang"
)

func TestDomainMessages(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New(i18n.Config{Locale: "en"}, lang.Files())
	require.NoError(t, err)

	keys := []string{}
	for _, code := range append(dns.WarningCodes(), email.WarningDomainClaimed) {
		keys = append(keys, "email_domains.warnings."+code)
	}

	for _, reason := range []string{
		email.FailureNotVerified, email.FailureRecordsRemoved, email.FailureOwnershipTransfered,
		email.FailureProviderRejected,
	} {
		keys = append(keys, "email_domains.failures."+reason)
	}

	for _, host := range dns.Hosts() {
		keys = append(keys, host.Steps...)
		keys = append(keys, host.Quirks...)
	}

	t.Run("explains every mistake, failure and host step in words", func(t *testing.T) {
		t.Parallel()

		for _, key := range keys {
			_, found := translator.Lookup(t.Context(), key, i18n.Args{"host": "acme.test", "value": "x"})
			assert.True(t, found, key)
		}
	})
}

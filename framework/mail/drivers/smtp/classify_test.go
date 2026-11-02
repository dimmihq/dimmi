package smtp

import (
	"errors"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	replies := map[string]struct {
		code int
		msg  string
		kind error
	}{
		"refuses a mailbox for good on 550":     {550, "5.1.1 User unknown", mail.ErrPermanent},
		"asks to try later on 450":              {450, "4.2.1 Mailbox busy", mail.ErrTemporary},
		"treats a refused login as temporary":   {535, "5.7.8 Authentication failed", mail.ErrTemporary},
		"asks to slow down on 421":              {421, "4.7.0 Try again later", &mail.RateLimitedError{}},
		"asks to slow down on a rate limit 451": {451, "4.7.1 Rate limit exceeded", &mail.RateLimitedError{}},
	}

	for name, reply := range replies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := classify("rcpt to", &textproto.Error{Code: reply.code, Msg: reply.msg})

			if _, ok := errors.AsType[*mail.RateLimitedError](reply.kind); ok {
				_, limited := errors.AsType[*mail.RateLimitedError](err)
				assert.True(t, limited)

				return
			}

			require.ErrorIs(t, err, reply.kind)
			assert.Contains(t, err.Error(), reply.msg)
		})
	}

	t.Run("treats a lost connection as temporary", func(t *testing.T) {
		t.Parallel()

		require.ErrorIs(t, classify("data", errors.New("broken pipe")), mail.ErrTemporary)
	})
}

func TestQueueID(t *testing.T) {
	t.Parallel()

	t.Run("reads the id servers put in the final reply", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "4ABC123", queueID("2.0.0 Ok: queued as 4ABC123"))
		assert.Equal(t, "abc@example.com", queueID("Ok queued as <abc@example.com>"))
		assert.Empty(t, queueID("2.0.0 Ok"))
	})
}

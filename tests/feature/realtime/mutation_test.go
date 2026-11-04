package realtime_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	appmiddleware "github.com/dimmihq/dimmi/app/http/middleware"
)

func TestClientMutationID(t *testing.T) {
	t.Parallel()

	request := func(value string) *fhttp.Request {
		return &fhttp.Request{Header: fhttp.Header{appmiddleware.ClientMutationIDHeader: {value}}}
	}

	t.Run("reads the id the client gave its change", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "m-1", appmiddleware.ClientMutationID(request(" m-1 ")))
		assert.Empty(t, appmiddleware.ClientMutationID(&fhttp.Request{Header: fhttp.Header{}}))
	})

	t.Run("cuts an id longer than the cap", func(t *testing.T) {
		t.Parallel()

		id := appmiddleware.ClientMutationID(request(strings.Repeat("x", 100)))

		assert.Len(t, id, appmiddleware.MaxClientMutationID)
	})
}

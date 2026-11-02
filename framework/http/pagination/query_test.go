package pagination_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http/pagination"
)

func parse(t *testing.T, raw string, allow pagination.Allow) (pagination.Query, error) {
	t.Helper()

	values, err := url.ParseQuery(raw)
	require.NoError(t, err)

	return pagination.Parse(values, allow)
}

func TestParse(t *testing.T) {
	t.Parallel()

	allow := pagination.Allow{
		Filters:  []string{"status", "owner"},
		Sorts:    []string{"created_at", "last_message_at"},
		Includes: []string{"author", "tags"},
	}

	t.Run("reads every parameter of the grammar", func(t *testing.T) {
		t.Parallel()
		q, err := parse(t, "filter[status]=open,paused&filter[status]=closed&filter[owner]=me"+
			"&sort=-last_message_at,created_at&include=author,tags&cursor=abc&limit=50&q=+refund+&upsert=true", allow)
		require.NoError(t, err)
		assert.Equal(t, pagination.Query{
			Filters: map[string][]string{"status": {"open", "paused", "closed"}, "owner": {"me"}},
			Sorts: []pagination.Sort{
				{Field: "last_message_at", Desc: true},
				{Field: "created_at"},
			},
			Includes: []string{"author", "tags"},
			Cursor:   "abc",
			Limit:    50,
			Search:   "refund",
		}, q)
	})

	t.Run("falls back to the default limit and nothing else", func(t *testing.T) {
		t.Parallel()
		q, err := parse(t, "sort=&include=", allow)
		require.NoError(t, err)
		assert.Equal(t, pagination.Query{Limit: pagination.DefaultLimit}, q)
	})

	t.Run("uses the endpoint's own limits", func(t *testing.T) {
		t.Parallel()
		own := pagination.Allow{DefaultLimit: 10, MaxLimit: 20}
		q, err := parse(t, "", own)
		require.NoError(t, err)
		assert.Equal(t, 10, q.Limit)

		q, err = parse(t, "limit=20", own)
		require.NoError(t, err)
		assert.Equal(t, 20, q.Limit)

		q, err = parse(t, "", pagination.Allow{DefaultLimit: 50, MaxLimit: 20})
		require.NoError(t, err)
		assert.Equal(t, 20, q.Limit)
	})

	t.Run("refuses a limit out of range", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{"limit=0", "limit=-1", "limit=101", "limit=ten", "limit="} {
			_, err := parse(t, raw, allow)
			require.ErrorIs(t, err, pagination.ErrInvalidLimit, raw)

			var f *fault.Error
			require.ErrorAs(t, err, &f)
			assert.Equal(t, map[string]any{"parameter": "limit", "max": 100}, f.Meta())
		}
	})

	t.Run("refuses names the endpoint does not allow", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			raw       string
			want      error
			parameter string
		}{
			{"filter[secret]=1&filter[status]=open", pagination.ErrInvalidFilter, "secret"},
			{"filter[]=1", pagination.ErrInvalidFilter, ""},
			{"sort=-password", pagination.ErrInvalidSort, "-password"},
			{"sort=-", pagination.ErrInvalidSort, "-"},
			{"include=author,secrets", pagination.ErrInvalidInclude, "secrets"},
		}
		for _, c := range cases {
			_, err := parse(t, c.raw, allow)
			require.ErrorIs(t, err, c.want, c.raw)
			assert.Equal(t, fault.Invalid, fault.KindOf(err))

			var f *fault.Error
			require.ErrorAs(t, err, &f)
			assert.Equal(t, c.parameter, f.Meta()["parameter"], c.raw)
		}
	})

	t.Run("reports the same bad filter every time", func(t *testing.T) {
		t.Parallel()
		for range 20 {
			_, err := parse(t, "filter[zeta]=1&filter[alpha]=1&filter[mid]=1", allow)

			var f *fault.Error
			require.ErrorAs(t, err, &f)
			assert.Equal(t, "alpha", f.Meta()["parameter"])
		}
	})

	t.Run("ignores parameters that only look like filters", func(t *testing.T) {
		t.Parallel()
		q, err := parse(t, "filter=open&filter[status=open", allow)
		require.NoError(t, err)
		assert.Nil(t, q.Filters)
	})
}

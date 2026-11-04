package arch_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
loadDomainConnectTemplate reads the template as plain JSON values, because
its format has its own field names.
*/
func loadDomainConnectTemplate(t *testing.T) (map[string]any, []map[string]any) {
	t.Helper()

	path := filepath.Join("..", "..", "deploy", "domainconnect", "trydimmi.com.email.json")
	contents, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)

	var template map[string]any
	require.NoError(t, json.Unmarshal(contents, &template))

	raw, ok := template["records"].([]any)
	require.True(t, ok, "records is a list")

	records := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		record, ok := entry.(map[string]any)
		require.True(t, ok, "every record is an object")
		records = append(records, record)
	}

	return template, records
}

func TestDeployDomainConnectTemplate(t *testing.T) {
	t.Parallel()

	template, records := loadDomainConnectTemplate(t)

	summary := func(record map[string]any) string {
		return fmt.Sprintf("%v %v", record["type"], record["host"])
	}

	t.Run("the template identifies dimmi and signs synchronous requests", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "trydimmi.com", template["providerId"])
		assert.Equal(t, "email", template["serviceId"])
		assert.Equal(t, false, template["syncBlock"])
		assert.NotEmpty(t, template["syncPubKeyDomain"])
	})

	t.Run("the records are exactly what ses sending needs", func(t *testing.T) {
		t.Parallel()

		got := make([]string, 0, len(records))
		for _, record := range records {
			got = append(got, summary(record))
		}

		assert.ElementsMatch(t, []string{
			"SPFM @",
			"TXT _dimmi-verification",
			"CNAME %dkim1%._domainkey",
			"CNAME %dkim2%._domainkey",
			"CNAME %dkim3%._domainkey",
			"MX %mailfrom%",
			"TXT %mailfrom%",
			"TXT _dmarc",
		}, got)
	})

	t.Run("only dmarc is essential on apply", func(t *testing.T) {
		t.Parallel()

		for _, record := range records {
			if record["host"] == "_dmarc" {
				assert.Equal(t, "OnApply", record["essential"])

				continue
			}

			assert.NotContains(t, record, "essential", summary(record))
		}
	})

	t.Run("no record needs a cname at the apex", func(t *testing.T) {
		t.Parallel()

		assert.False(t, slices.ContainsFunc(records, func(record map[string]any) bool {
			return record["type"] == "APEXCNAME"
		}))
	})

	t.Run("the variables are the ones the signed link fills in", func(t *testing.T) {
		t.Parallel()

		contents, err := json.Marshal(records)
		require.NoError(t, err)

		found := map[string]bool{}
		for _, match := range regexp.MustCompile(`%([a-z0-9]+)%`).FindAllStringSubmatch(string(contents), -1) {
			found[match[1]] = true
		}

		assert.Equal(t, map[string]bool{
			"dkim1": true, "dkim2": true, "dkim3": true, "mailfrom": true, "token": true, "region": true,
		}, found)
	})
}

/*
|--------------------------------------------------------------------------
| Embedded Data
|--------------------------------------------------------------------------
|
| Lists the binary carries so it needs no network or disk to judge input,
| one entry per line, compared without regard to case.
|
| The common passwords are the most used passwords seen in public breach
| data, kept only when long enough to pass the shortest length the
| application allows, since shorter ones are refused by length alone.
| The list is stored compressed to keep the binary small. The free mail
| domains are those anyone can get an address under, which no single
| company may claim as its own.
|
*/

package data

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed common_passwords.txt.gz
var commonPasswords []byte

/*
CommonPasswords returns the passwords too widely used to accept, in
lower case, as a set.
*/
func CommonPasswords() (map[string]struct{}, error) {
	reader, err := gzip.NewReader(bytes.NewReader(commonPasswords))
	if err != nil {
		return nil, fmt.Errorf("open common passwords: %w", err)
	}

	set := make(map[string]struct{}, 50_000)

	lines := bufio.NewScanner(reader)
	for lines.Scan() {
		if entry := strings.ToLower(strings.TrimSpace(lines.Text())); entry != "" {
			set[entry] = struct{}{}
		}
	}

	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("read common passwords: %w", err)
	}

	return set, nil
}

//go:embed freemail_domains.txt
var freemailDomains string

/*
FreemailDomains returns the domains anyone can get an address under,
in lower case, as a set. No single company can claim one.
*/
func FreemailDomains() map[string]struct{} {
	set := map[string]struct{}{}

	for line := range strings.SplitSeq(freemailDomains, "\n") {
		if entry := strings.ToLower(strings.TrimSpace(line)); entry != "" {
			set[entry] = struct{}{}
		}
	}

	return set
}

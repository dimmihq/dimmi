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
| The disposable email domains are addresses anyone can take for a few
| minutes, from https://github.com/disposable-email-domains/disposable-email-domains
| (disposable_email_blocklist.conf), dedicated to the public domain
| under CC0 1.0. The dev command data:disposable-domains refreshes it.
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

//go:embed disposable_email_domains.txt.gz
var disposableDomains []byte

/*
CommonPasswords returns the passwords too widely used to accept, in
lower case, as a set.
*/
func CommonPasswords() (map[string]struct{}, error) {
	return readSet(commonPasswords, "common passwords", 50_000)
}

/*
DisposableDomains returns the domains of throwaway mailboxes, in lower
case, as a set.
*/
func DisposableDomains() (map[string]struct{}, error) {
	return readSet(disposableDomains, "disposable domains", 12_000)
}

/*
readSet reads a compressed list, one entry per line, into a set of its
entries in lower case, skipping blank lines.
*/
func readSet(compressed []byte, name string, size int) (map[string]struct{}, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}

	set := make(map[string]struct{}, size)

	lines := bufio.NewScanner(reader)
	for lines.Scan() {
		if entry := strings.ToLower(strings.TrimSpace(lines.Text())); entry != "" {
			set[entry] = struct{}{}
		}
	}

	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
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

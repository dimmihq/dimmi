/*
|--------------------------------------------------------------------------
| Migration Lint
|--------------------------------------------------------------------------
|
| Servers are upgraded one at a time, so for a while the old and the new
| release share one schema. A migration may therefore only expand it:
| add tables, nullable columns and indexes. Anything that removes or
| rewrites what the old release relies on is a contract migration, and
| must say so in its header so it ships one release after the code that
| stopped using it.
|
| Some statements are legal but take a lock that stalls live traffic for
| as long as they run. The lint catches those before they reach a busy
| table. It reads the SQL as text and errs on the side of a finding.
|
*/

package migrate

import (
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

/*
Finding is one problem Lint found: the file, the rule and a message
saying what to do instead.
*/
type Finding struct {
	Path    string
	Rule    string
	Message string
}

/*
The lint rules.

RuleHeader wants a "-- migrate: phase=expand" or "phase=contract" line.
RuleContract flags DROP and RENAME in an expand migration; dropping an
index with DROP INDEX CONCURRENTLY IF EXISTS stays allowed. RuleColumnType
flags column type changes, which rewrite the table. RuleSetNotNull flags
SET NOT NULL unless an earlier migration added a NOT VALID check for the
column. RuleVolatileDefault flags new columns with a volatile default.
RuleBlockingIndex flags CREATE INDEX without CONCURRENTLY on a table the
migration did not create. RuleRead reports a file that cannot be read.
*/
const (
	RuleHeader          = "header"
	RuleContract        = "contract-in-expand"
	RuleColumnType      = "column-type"
	RuleSetNotNull      = "set-not-null"
	RuleVolatileDefault = "volatile-default"
	RuleBlockingIndex   = "blocking-index"
	RuleRead            = "read"
)

var (
	dropRe          = regexp.MustCompile(`(?i)\bDROP\b`)
	safeDropRe      = regexp.MustCompile(`(?i)\bDROP\s+INDEX\s+CONCURRENTLY\s+IF\s+EXISTS\b`)
	renameRe        = regexp.MustCompile(`(?i)\bRENAME\b`)
	columnTypeRe    = regexp.MustCompile(`(?i)\bALTER\s+(?:COLUMN\s+)?"?\w+"?\s+(?:SET\s+DATA\s+)?TYPE\b`)
	setNotNullRe    = regexp.MustCompile(`(?i)\bALTER\s+(?:COLUMN\s+)?"?(\w+)"?\s+SET\s+NOT\s+NULL\b`)
	notValidCheckRe = regexp.MustCompile(`(?i)\bCHECK\s*\(\s*"?(\w+)"?\s+IS\s+NOT\s+NULL\s*\)\s*NOT\s+VALID\b`)
	volatileRe      = regexp.MustCompile(`(?i)\bADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?"?\w+"?\s+[^,;]*?` +
		`(?:\bDEFAULT\s+(?:random|clock_timestamp|timeofday|gen_random_uuid|uuid_generate_v[14]|nextval)\s*\(` +
		`|\b(?:small|big)?serial\b)`)
	createIndexRe = regexp.MustCompile(`(?i)\bCREATE\s+(?:UNIQUE\s+)?INDEX\s+(CONCURRENTLY\b)?[^;]*?\bON\s+` +
		`(?:ONLY\s+)?([\w."]+)`)
	createTableRe = regexp.MustCompile(`(?i)\bCREATE\s+(?:UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w."]+)`)
)

/*
Lint checks the up section of every SQL migration at the root of the
filesystem, in file name order, and returns what it found.
*/
func Lint(fsys fs.FS) []Finding {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return []Finding{{Path: ".", Rule: RuleRead, Message: err.Error()}}
	}

	var findings []Finding

	guarded := map[string]bool{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		text, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			findings = append(findings, Finding{Path: entry.Name(), Rule: RuleRead, Message: err.Error()})

			continue
		}

		file := parseFile(entry.Name(), string(text))
		findings = append(findings, lintFile(file, guarded)...)

		for _, match := range notValidCheckRe.FindAllStringSubmatch(file.code, -1) {
			guarded[strings.ToLower(match[1])] = true
		}
	}

	return findings
}

/*
lintFile runs every rule on one file. Guarded holds the columns earlier
migrations protected with a NOT VALID check.
*/
func lintFile(file sqlFile, guarded map[string]bool) []Finding {
	var problems []Finding

	report := func(rule, format string, args ...any) {
		problems = append(problems, Finding{Path: file.path, Rule: rule, Message: fmt.Sprintf(format, args...)})
	}

	if file.phase == "" {
		report(RuleHeader, `add a "-- migrate: phase=expand" or "-- migrate: phase=contract" header`)
	}

	if file.phase == phaseExpand && hasContract(file.code) {
		report(RuleContract, "DROP and RENAME belong in a contract migration, shipped after the code stops using them")
	}

	if columnTypeRe.MatchString(file.code) {
		report(RuleColumnType, "changing a column type rewrites the table; add a new column and backfill it instead")
	}

	for _, match := range setNotNullRe.FindAllStringSubmatch(file.code, -1) {
		if !guarded[strings.ToLower(match[1])] {
			report(RuleSetNotNull, "validate CHECK (%s IS NOT NULL) NOT VALID in an earlier migration", match[1])
		}
	}

	if volatileRe.MatchString(file.code) {
		report(RuleVolatileDefault, "a volatile default rewrites the table; add the column without it and backfill")
	}

	for _, table := range blockingIndexes(file.code) {
		report(RuleBlockingIndex, "create the index on %s CONCURRENTLY in a NO TRANSACTION migration", table)
	}

	return problems
}

func hasContract(code string) bool {
	drops := len(dropRe.FindAllStringIndex(code, -1)) - len(safeDropRe.FindAllStringIndex(code, -1))

	return drops > 0 || renameRe.MatchString(code)
}

/*
blockingIndexes lists the tables that get an index without CONCURRENTLY,
leaving out tables the same code creates, which nothing else uses yet.
*/
func blockingIndexes(code string) []string {
	var created []string
	for _, match := range createTableRe.FindAllStringSubmatch(code, -1) {
		created = append(created, strings.ToLower(match[1]))
	}

	var tables []string

	for _, match := range createIndexRe.FindAllStringSubmatch(code, -1) {
		if match[1] == "" && !slices.Contains(created, strings.ToLower(match[2])) {
			tables = append(tables, match[2])
		}
	}

	return tables
}

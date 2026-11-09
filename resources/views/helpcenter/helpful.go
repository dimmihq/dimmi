/*
|--------------------------------------------------------------------------
| Article Helpfulness
|--------------------------------------------------------------------------
|
| Where a reader stands with "Was this article helpful?", carried in the
| page's address after each answer, so the plain forms work without
| scripts and a reload shows the same step.
|
*/

package helpcenter

/*
Helpful is where a reader stands with "Was this article helpful?": the
Answer they gave, yes or no, empty before they answer; Commented once
they said what was missing after a no; Limited when they sent too much
feedback in the last hour.
*/
type Helpful struct {
	Answer    string
	Commented bool
	Limited   bool
}

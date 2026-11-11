/*
|--------------------------------------------------------------------------
| Tool Catalog: Live Series
|--------------------------------------------------------------------------
|
| The entries for a series once it runs: publishing, pausing, resuming
| and archiving it, picking a split's winner, its report, and the people
| on it. Whatever changes what customers get asks the person first.
|
*/

package mcpserver

/*
seriesLiveDocs returns the entries for publishing, pausing, resuming,
archiving and picking winners.
*/
func seriesLiveDocs() map[string]Doc {
	return map[string]Doc{
		"series.publications.store": {
			Title: "Publish a series",
			Use: "Freezes the working copy as a new version and sets it live. People already on it follow the " +
				"new version from the step they stand on. Nothing may be missing; check it first. Needs the " +
				"permission to publish series.",
			Returns: "the series, now live.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"note": "What changed, for the history, up to 280 characters.",
				"moves": "For each removed step people stand on, by step id, the step id they move to. By " +
					"default they go to the first later step that still exists, or leave.",
			},
			Hint: "series_invalid means it misses something: list it with dimmi_series_check_create.",
		},
		"series.pause.store": {
			Title:   "Pause a series",
			Use:     "Stops every message and step of a live series at once; everyone stays where they stand.",
			Returns: "the series, now paused.",
			Risk:    new(RiskReach),
		},
		"series.resume.store": {
			Title:   "Resume a series",
			Use:     "Sets a paused series live again; waits that ended meanwhile carry on.",
			Returns: "the series, live again.",
			Risk:    new(RiskReach),
		},
		"series.archive.store": {
			Title: "Archive a series",
			Use: "Ends a series and every journey on it for good, withdrawing what was not sent yet. Its " +
				"versions and report stay.",
			Returns: "the series, now archived.",
			Risk:    new(RiskDanger),
		},
		"series.nodes.winner.store": {
			Title:   "Pick a split's winner",
			Use:     "Sends everyone who reaches a split step from now on down one variant.",
			Returns: "the series as it now is.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"node": "Id of the split step, from the spec dimmi_series_get returns.",
				"path": "The variant's path name, from the split's edges.",
			},
		},
	}
}

/*
seriesPeopleDocs returns the entries for the people on a series.
*/
func seriesPeopleDocs() map[string]Doc {
	return map[string]Doc{
		"series.enrollments.index": {
			Title:   "List a series' journeys",
			Use:     "Lists the people or accounts on a series, by the step they stand on, status or name.",
			Returns: "a page of journeys with who travels, their step and status.",
			Params: map[string]string{
				"node":   "Only journeys standing on this step id, from the spec.",
				"status": "active, waiting, finished or exited.",
				"q":      "Search on who travels, up to 100 characters.",
				"offset": "How many journeys to skip, for the next page.",
			},
		},
		"series.enrollments.show": {
			Title:   "Get a journey",
			Use:     "Reads one journey step by step: what each step did and when.",
			Returns: "the journey with its trace of steps.",
		},
		"series.enrollments.store": {
			Title: "Add people to a series",
			Use: "Adds contacts, or companies on an account series, to a live series by hand, under the same " +
				"rules as automatic entry. They start getting its messages.",
			Returns: "the journeys it started.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"contact_ids": "Contacts (ctc_...) to add, up to 100.",
				"company_ids": "Companies (cmp_...) to add to an account series, up to 100.",
			},
			Hint: "series_not_live means it must be published or resumed first.",
		},
		"series.enrollments.destroy": {
			Title:   "Take someone off a series",
			Use:     "Ends one journey now; nothing more of the series reaches them.",
			Returns: "nothing once it ended.",
		},
		"series.enrollments.skip.store": {
			Title:   "Skip a test journey's wait",
			Use:     "Ends the wait a test journey stands on now, so the next step runs. Only for test journeys.",
			Returns: "nothing once it moved on.",
			Hint:    "series_enrollment_not_test means it is a real journey, whose waits cannot be skipped.",
		},
	}
}

/*
journeyDocs returns the entries for a series' report, trying a series
and the journeys of one contact or company.
*/
func journeyDocs() map[string]Doc {
	return map[string]Doc{
		"series.report.show": {
			Title: "Get a series' report",
			Use: "Reads how many entered, are on it now, finished, left and why, and reached the goal; per day " +
				"and per step, with what each message step's broadcast counted. Test journeys never count.",
			Returns: "the totals, the daily series and the numbers of every step.",
			Params:  map[string]string{"days": "How many days back the daily numbers cover."},
		},
		"series.tests.store": {
			Title: "Try a series on yourself",
			Use: "Enrolls the contact with the person's own email in the series as a test journey: it may run " +
				"again and again, counts in no report and its waits can be skipped.",
			Returns: "the test journey step by step.",
			Next:    []string{"dimmi_series_enrollments_skip_create: skip a wait"},
			Hint:    "series_test_contact_missing means no contact has the person's email yet; add one first.",
		},
		"contacts.series.index": {
			Title:   "List a contact's series",
			Use:     "Lists the series a contact is or was on, and where they stand on each.",
			Returns: "the contact's journeys with their series, step and status.",
		},
		"companies.series.index": {
			Title:   "List a company's series",
			Use:     "Lists the account series a company is or was on, and where it stands on each.",
			Returns: "the company's journeys with their series, step and status.",
		},
	}
}

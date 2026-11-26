package mcpserver

/*
schemaTextsFromMigration returns the texts of the fields about
migrations from other tools, their records and their undo.
*/
func schemaTextsFromMigration() map[string]map[string]string {
	ref := map[string]string{
		"kind":      "The kind of the record named, such as contact or teammate.",
		"source_id": "The record's id in the other tool.",
	}

	return map[string]map[string]string{
		"Migration":              migrationTexts(),
		"MigrationSummary":       migrationSummaryTexts(),
		"MigrationKindCount":     migrationCountTexts(),
		"MigrationRollback":      migrationRollbackTexts(),
		"MigrationBatch":         migrationBatchTexts(),
		"MigrationRecordResult":  migrationResultTexts(),
		"MigrationRecordProblem": migrationProblemTexts(),
		"MigrationLookup": {
			"source_id": "The record's id in the other tool, as asked.",
			"status": "applied, deferred, failed or unsupported as the ledger has it, or missing when the " +
				"migration never received it.",
		},
		"MigrationError": {
			"code":    "A stable code for why the record failed, such as company_exists.",
			"field":   "The record field at fault, such as external_id. Null when no single field is.",
			"message": "What went wrong, in words for a person.",
		},
		"Notice": {
			"code":    "A stable code for the warning, such as not_supported_yet or file_unavailable.",
			"field":   "The record field left out, such as custom_attributes.",
			"message": "What was left out and why, in words for a person.",
		},
		"Conflict": {
			"field":  "The field a teammate changed in dimmi since the migration wrote it, such as status.",
			"dimmi":  "The value it holds in dimmi now, which the sync kept.",
			"source": "The value the other tool sent, which was not applied.",
		},
		"Ref": ref,
	}
}

/*
migrationTexts returns the texts of a migration's fields.
*/
func migrationTexts() map[string]string {
	out := migrationSummaryTexts()
	out["format_version"] = "The import format version the migration reads, 1 today."
	out["counts"] = "The migration's records per kind, by how they stand."
	out["problems"] = "The latest 50 records that failed, are unsupported or carry conflicts, newest first."

	return out
}

/*
migrationSummaryTexts returns the texts of a migration's fields in a
list.
*/
func migrationSummaryTexts() map[string]string {
	return map[string]string{
		"name":           "The migration's name, such as Intercom move. Null when none was given.",
		"source":         "The short name of the tool the history comes from, such as intercom.",
		"source_account": "The id of the account in that tool. Starting again for it resumes this migration.",
		"status": "active while it takes records, rolling_back while it is undone, rolled_back once " +
			"undone, or rollback_failed when the undo stopped and can be asked again.",
		"last_record_at": "When the migration last received a record. Null before the first.",
	}
}

/*
migrationCountTexts returns the texts of one kind's counts.
*/
func migrationCountTexts() map[string]string {
	return map[string]string{
		"kind":        "The record kind counted, such as conversation.",
		"applied":     "Records applied, created or matched to what the workspace had.",
		"created":     "Applied records the migration created, which an undo removes.",
		"deferred":    "Records waiting for others they name.",
		"failed":      "Records not applied because of an error.",
		"unsupported": "Records dimmi cannot apply yet.",
		"conflicted":  "Records whose last sync left a teammate's change in place.",
	}
}

/*
migrationRollbackTexts returns the texts of a migration's undo.
*/
func migrationRollbackTexts() map[string]string {
	return map[string]string{
		"requested_at": "When the undo was asked for.",
		"finished_at":  "When the undo finished or stopped. Null while it runs.",
		"removed":      "How many records the undo removed so far.",
		"error":        "Why the undo stopped. Null unless it failed.",
	}
}

/*
migrationBatchTexts returns the texts of a batch's answer.
*/
func migrationBatchTexts() map[string]string {
	return map[string]string{
		"kind":     "The kind of the records sent.",
		"results":  "A result per record, in the order sent.",
		"resolved": "Records sent before that waited for this batch and are now applied or failed.",
	}
}

/*
migrationResultTexts returns the texts of one record's result.
*/
func migrationResultTexts() map[string]string {
	return map[string]string{
		"kind":      "The record's kind.",
		"source_id": "The record's id in the other tool.",
		"status": "created, updated, unchanged (sent before as it is and skipped), deferred, failed or " +
			"unsupported.",
		"warnings":    "Fields left out of the record, each with a code and why.",
		"conflicts":   "Fields a teammate changed in dimmi, which this sync left as they are.",
		"waiting_for": "The records a deferred record waits for. Empty otherwise.",
	}
}

/*
migrationProblemTexts returns the texts of one problem record.
*/
func migrationProblemTexts() map[string]string {
	return map[string]string{
		"kind":      "The record's kind.",
		"source_id": "The record's id in the other tool.",
		"status":    "failed or unsupported; applied when only conflicts make it a problem.",
		"conflicts": "Fields a teammate changed in dimmi, which the last sync left as they are.",
	}
}

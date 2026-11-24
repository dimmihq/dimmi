<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "review_procedures" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: the order of tool calls that does this kind of work well in few steps, what
to skip, and how to name each suggestion so no run suggests it twice.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to review procedures.

1. Call list_procedures, and for each live procedure call list_procedure_anomalies.
2. Read the runs that went wrong with read_procedure_trace, and the procedure with get_procedure_by_id.
3. Call list_recent_suggestions so you never repeat a waiting or declined suggestion.
4. When a step is unclear or keeps failing, call propose_changes with procedure.edit_draft fixing
   it and a procedure.create_test_case from a failing conversation, with a dedup_key like
   qa:<procedure id>:<step>. One procedure per call.
5. When every procedure holds up and the routine asks for nothing else, call report_nothing_to_change with the
   reason.

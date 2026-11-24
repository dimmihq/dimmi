<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "find_knowledge_gaps" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: the order of tool calls that does this kind of work well in few steps, what
to skip, and how to name each suggestion so no run suggests it twice.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to find knowledge gaps.

1. Call get_routine_state to see the topics earlier runs already covered.
2. Call get_workspace_report for the run's dates to see handoff reasons, top tags and the least helpful
   articles.
3. Call list_conversations with ai_agent handed_off and status any, and read up to 10 of the most
   telling
   with get_conversation_by_number and get_ai_agent_runs, passing each conversation's number.
4. Group the questions that went unanswered: at least 2 conversations each, or 1 about billing or
   policy, or an article with 2 or more "not helpful" votes.
5. Before suggesting anything, call search_knowledge, list_articles and list_recent_suggestions. Never
   suggest what an article, a draft, a waiting suggestion or one declined lately already covers.
6. For each real gap, at most 5, suggest one change with a dedup_key like gap:<topic-slug>:
   - propose_article_edits when an article exists but is unclear or incomplete;
   - propose_new_article with visibility public for a customer guide, or agent_only for a short
     policy or edge-case snippet only the AI agent reads.
   Keep every customer detail out of the article. Cite conversation numbers only in the reason.
7. Save the end of the run's window and the topics you covered with save_routine_state.
8. When there is no gap, call report_nothing_to_change with the reason.

<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "audit_articles" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: the order of tool calls that does this kind of work well in few steps, what
to skip, and how to name each suggestion so no run suggests it twice.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to audit help articles.

When you are the main agent:
1. Call get_routine_state for next_offset; with none saved, start at 0.
2. Call list_articles once with offset next_offset and limit 100. It says how many articles the
   help center has. When it lists none, the last run reached the end: list once more from
   offset 0. Never list a third time.
3. With more than 25 listed, split them: call run_subagents with up to 4 subagents, each given
   25 articles by their place in the list. Give each this task, with its own numbers: "Audit the
   help articles at offset 25, limit 25, for contradictions and stale facts. Read the
   audit_articles skill first and follow its steps for a subagent." With 25 or fewer, audit them
   yourself with the subagent's steps below.
4. Save next_offset with save_routine_state: the offset you listed from plus the number of
   articles audited this run.
5. When nobody suggested anything, call report_nothing_to_change with the reason.

When you are a subagent, or auditing 25 or fewer yourself:
1. In one step, call list_articles with the offset and limit you were given, and
   list_recent_suggestions so you never repeat a waiting or declined suggestion.
2. In one step, read every article with get_article_by_id.
3. In one step, compare their facts with the rest of the help center using search_knowledge:
   refund windows, limits, prices, plan names, steps. Search once per fact worth checking, not
   once per article: about 15 searches is plenty, and your tool calls are limited. Read another
   article only when a search result is not enough to tell.
4. Call propose_article_edits once, with one fix for each article that has a contradiction or
   stale fact: the article's id, edits with the exact sentences to fix, and a dedup_key like
   stale:<article id>. Name the conflicting article by its title in the reason. A fix must
   change the text; what only needs a person to look at goes in your report, not in a fix.

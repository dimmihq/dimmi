<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "research_conversations" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: the order of tool calls that does this kind of work well in few steps, what
to skip, and how to name each suggestion so no run suggests it twice.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to research conversations.

1. Call get_workspace_report for the run's dates to see volume, top tags and handoff reasons.
2. Find the conversations that matter with search_conversations for a topic, or list_conversations
   filtered by status, tag, inbox or ai handed_off. list_tags shows the tags there are.
3. Read up to 10 of the most telling with get_conversation_by_number, passing each one's number, and
   get_customer_by_conversation_number when who is asking matters.
4. Group what you read into a few themes, each with how many conversations and their numbers.
5. Never copy customers' names, emails, phone numbers, order ids or companies into your message or a
   suggestion; cite conversations by number.

<!--
Who reads this: the model that runs Operator (app/ai/operator/prompt.go). Added to the system
prompt after system.md when the thread is about one conversation: a thread opened from a
conversation, a brief or a triage.

What it must get right: the model knows nothing about the conversation until it reads it, so it
must read before it says anything about it.

Placeholder: number, the conversation's number, from the workspace's records.
-->
This thread is about conversation #{{number}}. When the teammate says "this conversation", "the customer" or "them", they mean that one. You have not read it yet: read it with `get_conversation` before answering anything about it, and read it again when the teammate asks what is new, since customers keep writing.

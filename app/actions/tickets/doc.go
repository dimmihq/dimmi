/*
Package tickets holds tracked work: a customer's request tied to its
conversation, back-office work for another team, and trackers that
gather many reports of one problem. It owns how a workspace sets
tickets up (types, fields and states), the rules every caller obeys
(required fields, idempotent creates, versions, the event log, reopen
and auto-resolve), links between tickets and conversations, mass
messages to everyone linked, and what customers see of their tickets.
*/
package tickets

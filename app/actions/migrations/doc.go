/*
Package migrations brings a workspace's history in from another tool
through dimmi's own import format.

A client, usually the dimmi-migrate CLI on the customer's machine, reads
the other tool with the customer's own credentials, translates what it
reads into the records of [FormatVersion], and sends them in batches.
The server never learns which tool the records came from. Each record
is kept in a ledger under the id it had there, so sending it again
updates what it became instead of adding another, an unchanged record
is skipped, a record whose references have not arrived yet waits for
them, and an undo removes everything the migration created.

Everything is written as imported history: nothing is sent, and no
event goes out, so no workflow, notification, SLA, webhook or AI agent
reacts to it.
*/
package migrations

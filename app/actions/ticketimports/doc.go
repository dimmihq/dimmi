/*
Package ticketimports brings tickets in from Intercom and Zendesk: the
types and their fields, the states with the labels customers saw, the
tickets themselves with their history, and the trackers problems
gathered. It writes through the tickets writer, replaying history at
the time it happened without setting anything off, finds what an
earlier run brought in by the id it had in the other tool, and can be
undone. The importer that owns a workspace's connection to the other
tool calls it with a Source.
*/
package ticketimports

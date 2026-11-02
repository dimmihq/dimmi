/*
Package idempotency remembers the response to a request sent with an
idempotency key, so a client that retries gets the same answer instead
of doing the work twice.
*/
package idempotency

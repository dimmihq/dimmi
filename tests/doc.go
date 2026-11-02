/*
Package tests boots the real application for integration tests: one
fresh database copied from a migrated template, the shared Valkey under
a key prefix of the test's own, and a workspace with its owner.
*/
package tests

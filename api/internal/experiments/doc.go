// Package experiments is Chapter 32: deciding which variant a user sees, and
// recording that decision so the result can be trusted later.
//
//	assignment.go  the pure function that decides
//	service.go     the service around it: cache, and the audit write
//
// # Why assignment is a hash and not a coin flip
//
// A coin flip per request would move a user between variants on every page
// load: they would see the new board, then the old one, then the new one, and
// the experiment would be measuring nothing at all. Assignment has to be a
// function OF THE USER — same user, same experiment, same answer, forever,
// with no clock, no randomness and no I/O. That is twenty lines and the most
// important twenty lines in the package.
//
// The null byte between the two hashed inputs is not decoration.
// assignment_test.go says what it prevents: without a separator, experiment
// "abc" + user "def" and experiment "ab" + user "cdef" are the same byte
// string, so two unrelated experiments share a bucket assignment.
//
// # Why the audit row is written off the critical path
//
// The hash answers instantly; the assignment row is the audit trail, not the
// answer. Nobody's page load should wait on a row that exists to answer a
// question somebody will ask next month.
//
// [verbatim ch32]
package experiments

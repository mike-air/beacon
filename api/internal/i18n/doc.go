// Package i18n is Chapter 33: making the same data readable to someone who is
// not you. Three separate problems live here and only the first is about
// language.
//
//	locale.go    which language?   a four-source cascade, most specific first
//	messages.go  what does it say? a catalog keyed by the English string
//	time.go      when, and how much money?
//
// # Language
//
// The cascade is: the user's stored locale, then their organisation's
// default, then Accept-Language, then English. Most specific first, and the
// header comes third rather than first because a user who has told you their
// language should not be overridden by their browser. It never returns the
// zero value — the worst case is English, which is readable.
//
// # Messages
//
// The English source string is BOTH the catalog key and the fallback. Nothing
// is ever rendered as `errors.welcome_message` or as a blank space — a string
// nobody has translated yet comes out in English, which is a perfectly
// readable answer and the reason a missing translation is never an incident.
//
// # Time and money, the two that look easy
//
// TIME: store UTC, always. timestamptz in the database, time.Now().UTC() in
// Go, and convert to the user's zone at the very last moment before a human
// reads it. Never store a local time and its zone in separate columns: "02:30
// on the spring-forward day in Berlin" is not a moment that happened, and no
// care downstream recovers it.
//
// MONEY: never float64. 0.1 + 0.2 is 0.30000000000000004, and that error
// compounds across a million rows. Store an integer count of minor units plus
// a currency, and let a library that knows currencies do the arithmetic.
//
// [verbatim ch33]
package i18n

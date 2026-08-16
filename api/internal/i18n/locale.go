// Package i18n is Chapter 33: making the same data readable to someone who is
// not you. Three separate problems live here, and only the first is about
// language.
//
//  1. Which language? A four-source cascade, most specific first.
//  2. How much money? Never a float. Minor units and a currency, together.
//  3. What time? UTC in the database, the user's zone at the last moment.
//
// [verbatim ch33]
package i18n

import (
	"context"
	"net/http"

	"golang.org/x/text/language"
)

// Supported is the set of locales we ship translations for. New locales
// are added here once the translation files exist.
var Supported = []language.Tag{
	language.English,  // en
	language.German,   // de
	language.Spanish,  // es
	language.French,   // fr
	language.Japanese, // ja
}

// The matcher narrows whatever was asked for to the nearest thing we actually
// have: de-AT becomes de; pt-BR, with no Portuguese shipped, becomes English.
var matcher = language.NewMatcher(Supported)

type localeCtxKey struct{}

// Prefs is what the request's user asked for, read once per request from the
// database. Any field may be empty; the cascade handles that.
//
// [glue: the chapter reads these off auth.UserFrom(ctx). This repo's context
// carries only the user ID (Chapter 16), so the middleware loads the row and
// puts this struct in the context instead.]
type Prefs struct {
	UserLocale string
	OrgLocale  string
	Timezone   string
}

// LocaleFrom resolves the request's locale by walking the four-source
// cascade. It never returns the zero value — the worst case is English.
func LocaleFrom(ctx context.Context, r *http.Request) language.Tag {
	// Kept for the chi path. The whole decision depends on one header, so the
	// real implementation takes that header and both transports share it —
	// two copies of a cascade would eventually disagree about what language a
	// reader gets, and nobody would notice until a customer complained.
	return LocaleFromHeader(ctx, r.Header.Get("Accept-Language"))
}

// LocaleFromHeader resolves the cascade: the user's stored locale, then their
// organisation's default, then Accept-Language, then English.
func LocaleFromHeader(ctx context.Context, acceptLanguage string) language.Tag {
	if p, ok := PrefsFrom(ctx); ok {
		if p.UserLocale != "" {
			if tag, err := language.Parse(p.UserLocale); err == nil {
				return matched(tag)
			}
		}
		if p.OrgLocale != "" {
			if tag, err := language.Parse(p.OrgLocale); err == nil {
				return matched(tag)
			}
		}
	}

	if header := acceptLanguage; header != "" {
		tags, _, err := language.ParseAcceptLanguage(header)
		if err == nil && len(tags) > 0 {
			tag, _, _ := matcher.Match(tags...)
			return tag
		}
	}

	return language.English
}

func matched(tag language.Tag) language.Tag {
	matched, _, _ := matcher.Match(tag)
	return matched
}

// WithPrefs / PrefsFrom carry the user's stored preferences through a request.
func WithPrefs(ctx context.Context, p Prefs) context.Context {
	return context.WithValue(ctx, prefsCtxKey{}, p)
}

func PrefsFrom(ctx context.Context) (Prefs, bool) {
	p, ok := ctx.Value(prefsCtxKey{}).(Prefs)
	return p, ok
}

type prefsCtxKey struct{}

// WithLocale / LocaleFromCtx carry the resolved tag, so a handler that needs a
// printer doesn't re-run the cascade.
func WithLocale(ctx context.Context, tag language.Tag) context.Context {
	return context.WithValue(ctx, localeCtxKey{}, tag)
}

func LocaleFromCtx(ctx context.Context) language.Tag {
	if tag, ok := ctx.Value(localeCtxKey{}).(language.Tag); ok {
		return tag
	}
	return language.English
}

// TimezoneFromCtx returns the user's IANA zone, defaulting to UTC.
func TimezoneFromCtx(ctx context.Context) string {
	if p, ok := PrefsFrom(ctx); ok && p.Timezone != "" {
		return p.Timezone
	}
	return "UTC"
}

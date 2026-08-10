package i18n

import (
	"testing"
	"time"

	"golang.org/x/text/language"
)

// The chapter's claim about zones: a stored UTC instant plus an IANA zone name
// survives the clocks changing. The same wall-clock offset does not.
func TestFormatTimeAcrossDST(t *testing.T) {
	// Two instants six months apart. Berlin is UTC+1 in January and UTC+2 in
	// July, and nothing in the stored value changes to say so — the zone rule
	// does the work.
	winter := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	summer := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	if got, want := FormatTime(winter, "Europe/Berlin", language.German), "15.01.2026, 13:00"; got != want {
		t.Errorf("winter: got %q want %q", got, want)
	}
	if got, want := FormatTime(summer, "Europe/Berlin", language.German), "15.07.2026, 14:00"; got != want {
		t.Errorf("summer: got %q want %q", got, want)
	}
}

func TestFormatTimeFallsBackToUTC(t *testing.T) {
	// An unknown zone must not panic or silently invent an offset.
	instant := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	if got, want := FormatTime(instant, "Middle/Earth", language.English), "Jul 15, 2026 at 12:00 PM"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestFormatTimePerLocale(t *testing.T) {
	instant := time.Date(2026, 5, 23, 14, 30, 0, 0, time.UTC)
	cases := map[language.Tag]string{
		language.English:  "May 23, 2026 at 2:30 PM",
		language.German:   "23.05.2026, 14:30",
		language.French:   "23/05/2026 à 14:30",
		language.Japanese: "2026年5月23日 14:30",
	}
	for tag, want := range cases {
		if got := FormatTime(instant, "UTC", tag); got != want {
			t.Errorf("%s: got %q want %q", tag, got, want)
		}
	}
}

// Money's whole reason for existing: a float would get this wrong and say
// nothing about it.
func TestMoneyIsNotFloat(t *testing.T) {
	// 0.1 + 0.2 in float64 is 0.30000000000000004. In minor units it is 30.
	sum, err := Add(10, "USD", 20, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Amount() != 30 {
		t.Errorf("got %d minor units, want 30", sum.Amount())
	}
	if got, want := FormatMoney(30, "USD"), "$0.30"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestMixingCurrenciesIsAnError(t *testing.T) {
	if _, err := Add(1000, "USD", 1000, "EUR"); err == nil {
		t.Fatal("adding USD to EUR should be an error, not a number")
	}
}

func TestZeroDecimalCurrency(t *testing.T) {
	// Not every currency has two decimal places. 1900 yen is ¥1,900, not ¥19.
	if got, want := FormatMoney(1900, "JPY"), "¥1,900"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

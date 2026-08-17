// Time formatting in the user's zone, and money as minor units plus a currency.
//
// [verbatim ch33] The two rules these functions exist to enforce — UTC in the
// database, and never a float for money — are argued in doc.go.

package i18n

import (
	"time"

	"github.com/Rhymond/go-money"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// FormatTime renders t in the user's locale and zone.
// Example: FormatTime(time.Date(2026, 5, 23, 14, 30, 0, 0, time.UTC),
//
//	"Europe/Berlin", language.German)
//	  -> "23.05.2026, 16:30"
func FormatTime(t time.Time, tz string, tag language.Tag) string {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	local := t.In(loc)
	return message.NewPrinter(tag).Sprintf("%v", local.Format(localeFormat(tag)))
}

func localeFormat(tag language.Tag) string {
	base, _ := tag.Base()
	switch base.String() {
	case "de":
		return "02.01.2006, 15:04"
	case "fr":
		return "02/01/2006 à 15:04"
	case "ja":
		return "2006年1月2日 15:04"
	case "es":
		return "02/01/2006 a las 15:04"
	default:
		return "Jan 2, 2006 at 3:04 PM"
	}
}

// FormatMoney renders an amount held as minor units + currency code.
//
// The signature is the lesson: there is no way to call this with a bare number,
// because a bare number is not an amount of money. go-money makes the currency
// part of the value, so mixing currencies returns an error instead of a wrong
// total.
func FormatMoney(minorUnits int64, currency string) string {
	return money.New(minorUnits, currency).Display()
}

// Add is here to show the failure mode the type buys you: adding USD to EUR is
// an error you cannot ignore, not a number you cannot trust.
func Add(aUnits int64, aCurrency string, bUnits int64, bCurrency string) (*money.Money, error) {
	return money.New(aUnits, aCurrency).Add(money.New(bUnits, bCurrency))
}

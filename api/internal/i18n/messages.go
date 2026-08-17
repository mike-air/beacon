// The message catalog: a map per locale, looked up by the English source
// string, falling back to that same string when a translation is missing.
//
// [verbatim ch33] Why the key is the English text, and not an identifier, is in
// doc.go — it is the reason a missing translation is never an incident.

package i18n

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func init() {
	// Source strings are written in English in the Go source. The
	// catalog maps each English source to its translations.
	_ = message.SetString(language.German, "Welcome to Beacon", "Willkommen bei Beacon")
	_ = message.SetString(language.German, "%d tasks completed", "%d Aufgaben erledigt")
	_ = message.SetString(language.Spanish, "Welcome to Beacon", "Bienvenido a Beacon")
	_ = message.SetString(language.Spanish, "%d tasks completed", "%d tareas completadas")
	_ = message.SetString(language.French, "Welcome to Beacon", "Bienvenue sur Beacon")
	_ = message.SetString(language.French, "%d tasks completed", "%d tâches terminées")
	_ = message.SetString(language.Japanese, "Welcome to Beacon", "Beacon へようこそ")
	_ = message.SetString(language.Japanese, "%d tasks completed", "%d 件のタスクが完了しました")
	// ... and so on for every translatable string
}

// PrinterFor returns a printer that formats strings in the given locale.
// The printer falls back to the source (English) for strings without a
// translation.
func PrinterFor(tag language.Tag) *message.Printer {
	return message.NewPrinter(tag)
}

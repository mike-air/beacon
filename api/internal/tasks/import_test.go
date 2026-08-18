// Unit tests for CSV parsing. No database — ParseCSV is pure, which is the
// reason it is a separate function from Import: the half of bulk import with
// all the fiddly cases is the half that can be tested in microseconds.
//
// The transactional half is covered by the integration tests in
// import_integration_test.go, which need a real Postgres to prove the thing
// worth proving: that a bad row leaves nothing behind.
package tasks_test

import (
	"errors"
	"strings"
	"testing"

	"beacon/internal/tasks"
)

func TestParseCSVHappyPath(t *testing.T) {
	rows, rowErrs, err := tasks.ParseCSV(strings.NewReader(
		"title,status,position\n" +
			"Fix the login bug,todo,1000\n" +
			"Ship the invoice screen,in_progress,2000\n",
	))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("row errors = %v, want none", rowErrs)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Title != "Fix the login bug" {
		t.Errorf("title = %q", rows[0].Title)
	}
	if rows[1].Status != tasks.StatusInProgress {
		t.Errorf("status = %q, want in_progress", rows[1].Status)
	}
	if rows[0].Position != 1000 {
		t.Errorf("position = %v, want 1000", rows[0].Position)
	}
}

// Only title is mandatory. A one-column file is the most common thing anyone
// pastes, so it has to work without ceremony.
func TestParseCSVTitleOnly(t *testing.T) {
	rows, rowErrs, err := tasks.ParseCSV(strings.NewReader("title\nJust this\n"))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("row errors = %v, want none", rowErrs)
	}
	if len(rows) != 1 || rows[0].Title != "Just this" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Status != tasks.StatusTodo {
		t.Errorf("status = %q, want the todo default", rows[0].Status)
	}
}

// Real exports capitalise headers however they like, and spreadsheets write
// "In Progress" where the API means in_progress. Both are normalised rather
// than rejected, because neither difference is one a user considers real.
func TestParseCSVNormalisesHeadersAndStatuses(t *testing.T) {
	rows, rowErrs, err := tasks.ParseCSV(strings.NewReader(
		"Title, Status\nShip it,In Progress\nDone thing,DONE\n",
	))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("row errors = %v, want none", rowErrs)
	}
	if rows[0].Status != tasks.StatusInProgress {
		t.Errorf("row 0 status = %q, want in_progress", rows[0].Status)
	}
	if rows[1].Status != tasks.StatusDone {
		t.Errorf("row 1 status = %q, want done", rows[1].Status)
	}
}

// The point of collecting rather than short-circuiting: one upload should
// report every problem, so the user edits their file once.
func TestParseCSVCollectsEveryRowError(t *testing.T) {
	_, rowErrs, err := tasks.ParseCSV(strings.NewReader(
		"title,status,position\n" +
			",todo,1\n" + // line 2: missing title
			"Fine,nonsense,2\n" + // line 3: bad status
			"Also fine,todo,abc\n", // line 4: bad position
	))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rowErrs) != 3 {
		t.Fatalf("row errors = %d (%v), want 3", len(rowErrs), rowErrs)
	}

	// Line numbers must match what the user sees in their editor: 1-based and
	// counting the header. An off-by-one here makes every message useless.
	wantLines := []int{2, 3, 4}
	wantCols := []string{"title", "status", "position"}
	for i, re := range rowErrs {
		if re.Line != wantLines[i] {
			t.Errorf("error %d line = %d, want %d", i, re.Line, wantLines[i])
		}
		if re.Column != wantCols[i] {
			t.Errorf("error %d column = %q, want %q", i, re.Column, wantCols[i])
		}
	}
}

func TestParseCSVRejectsMissingTitleColumn(t *testing.T) {
	_, _, err := tasks.ParseCSV(strings.NewReader("status,position\ntodo,1\n"))
	if err == nil {
		t.Fatal("want an error for a CSV with no title column")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Errorf("error = %q, should name the missing column", err)
	}
}

func TestParseCSVEmptyInputs(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"header only", "title\n"},
		{"header and a blank line", "title\n\n"},
		{"nothing at all", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tasks.ParseCSV(strings.NewReader(tc.in))
			if !errors.Is(err, tasks.ErrNoRows) {
				t.Errorf("err = %v, want ErrNoRows", err)
			}
		})
	}
}

// A file saved by a hand-editor usually ends in a newline. That must not read
// as an empty final task.
func TestParseCSVIgnoresTrailingBlankLine(t *testing.T) {
	rows, rowErrs, err := tasks.ParseCSV(strings.NewReader("title\nOne\nTwo\n\n"))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("row errors = %v, want none — a trailing newline is not a row", rowErrs)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
}

// A short row is a row error with a line number, not a file-level parse
// failure — that is why the reader's ragged-row check is disabled.
func TestParseCSVShortRowIsARowError(t *testing.T) {
	rows, rowErrs, err := tasks.ParseCSV(strings.NewReader(
		"title,status\nHas both,todo\nMissing its status\n",
	))
	if err != nil {
		t.Fatalf("ParseCSV should not fail the whole file for a short row: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("row errors = %v; a missing optional column is not an error", rowErrs)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[1].Status != tasks.StatusTodo {
		t.Errorf("status = %q, want the todo default", rows[1].Status)
	}
}

func TestParseCSVTitleLengthLimit(t *testing.T) {
	long := strings.Repeat("a", 201)
	_, rowErrs, err := tasks.ParseCSV(strings.NewReader("title\n" + long + "\n"))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rowErrs) != 1 {
		t.Fatalf("row errors = %d, want 1", len(rowErrs))
	}
	// The message should say how long it actually was — "too long" alone
	// makes the user count characters themselves.
	if !strings.Contains(rowErrs[0].Message, "201") {
		t.Errorf("message = %q, should report the actual length", rowErrs[0].Message)
	}
}

func TestParseCSVRowLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("title\n")
	for i := 0; i <= tasks.MaxImportRows; i++ {
		b.WriteString("task\n")
	}
	_, _, err := tasks.ParseCSV(strings.NewReader(b.String()))
	if !errors.Is(err, tasks.ErrTooManyRows) {
		t.Errorf("err = %v, want ErrTooManyRows", err)
	}
}

// Quoted commas are the whole reason to use a CSV reader instead of
// strings.Split, so the behaviour is worth a test that would catch a
// well-meaning "simplification" later.
func TestParseCSVHandlesQuotedCommas(t *testing.T) {
	rows, rowErrs, err := tasks.ParseCSV(strings.NewReader(
		"title,status\n\"Fix login, then logout\",todo\n",
	))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("row errors = %v", rowErrs)
	}
	if rows[0].Title != "Fix login, then logout" {
		t.Errorf("title = %q, want the comma preserved", rows[0].Title)
	}
}

package tasks

// Bulk import from CSV.
//
// The whole file exists to answer one question honestly: what happens when row
// 40 of 100 is wrong? The answer here is that NOTHING is written — the import
// is one transaction, and a single bad row rolls the whole thing back with a
// report saying which rows were bad and why.
//
// That is the boring choice, and it is the right one for a tool people paste
// spreadsheets into. The alternative — write the good rows, report the bad —
// reads as friendlier right up until someone fixes their CSV and re-uploads,
// because nothing in this schema can tell a retried row from a genuinely new
// one. There is no natural key on a task: two people can legitimately want two
// cards both called "Fix the login bug". So a partial import leaves the user
// with no safe move: re-upload and silently double half the board, or hand-
// edit the CSV down to the rows that failed. All-or-nothing means the fix is
// always the same one thing — correct the file, upload it again.
//
// The parse is deliberately separate from the write (ParseCSV, then Import),
// because they fail for different reasons and a caller may want the first
// without the second. Everything ParseCSV can reject, it rejects before a
// transaction is ever opened.

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"beacon/internal/audit"
	"beacon/internal/db"
)

// MaxImportRows caps one import. A bulk endpoint with no ceiling is a way to
// hold a transaction — and the row locks it takes — open for as long as the
// caller likes. 1000 is far more than anyone pastes by hand and small enough
// that the whole insert stays well inside a normal request.
const MaxImportRows = 1000

// ErrTooManyRows is returned when a CSV exceeds MaxImportRows.
var ErrTooManyRows = fmt.Errorf("a single import is limited to %d rows", MaxImportRows)

// ErrNoRows is returned for a CSV with a header and nothing under it. It is
// its own error because "your file was empty" and "your file was malformed"
// send the user to different places.
var ErrNoRows = errors.New("no data rows found")

// ImportRow is one parsed, validated row waiting to be written.
type ImportRow struct {
	Title    string
	Status   string
	Position float64
}

// RowError is one row that failed validation. Line is the line number in the
// uploaded file (1-based, counting the header), so it matches what the user
// sees in their editor rather than an internal slice index.
type RowError struct {
	Line    int    `json:"line"`
	Column  string `json:"column,omitempty"`
	Message string `json:"message"`
}

func (e RowError) Error() string {
	if e.Column != "" {
		return fmt.Sprintf("line %d, column %q: %s", e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

// ParseCSV reads a CSV into rows, collecting EVERY problem rather than
// stopping at the first.
//
// Collecting them all is the point: a user fixing a spreadsheet wants the
// whole list, not to re-upload five times to discover five typos. Only when
// the file is structurally unreadable (bad quoting, ragged rows) does this
// return early, because after that point the row numbers would be fiction.
//
// Required header: title. Optional: status, position. Header matching is
// case-insensitive and tolerates surrounding spaces, because every spreadsheet
// export capitalises differently and none of that is worth an error.
func ParseCSV(r io.Reader) ([]ImportRow, []RowError, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	// -1 disables the ragged-row check so a short row becomes a row error we
	// can report by line, rather than an opaque parse failure for the file.
	cr.FieldsPerRecord = -1

	records, err := cr.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("could not read CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, ErrNoRows
	}

	titleIdx, statusIdx, positionIdx := -1, -1, -1
	for i, h := range records[0] {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "title":
			titleIdx = i
		case "status":
			statusIdx = i
		case "position":
			positionIdx = i
		}
	}
	if titleIdx == -1 {
		return nil, nil, errors.New(`CSV needs a "title" column`)
	}

	body := records[1:]
	if len(body) == 0 {
		return nil, nil, ErrNoRows
	}
	if len(body) > MaxImportRows {
		return nil, nil, ErrTooManyRows
	}

	rows := make([]ImportRow, 0, len(body))
	var rowErrs []RowError

	for i, rec := range body {
		line := i + 2 // +1 for the header, +1 because humans count from one.

		// A trailing newline in a hand-edited file shows up as a one-field
		// record holding "". Skipping it silently is kinder than telling
		// someone their last line is empty when their editor put it there.
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue
		}

		at := func(idx int) string {
			if idx == -1 || idx >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[idx])
		}

		row := ImportRow{Title: at(titleIdx), Status: at(statusIdx)}

		switch {
		case row.Title == "":
			rowErrs = append(rowErrs, RowError{Line: line, Column: "title", Message: "title is required"})
		case len(row.Title) > 200:
			rowErrs = append(rowErrs, RowError{
				Line: line, Column: "title",
				Message: fmt.Sprintf("title is %d characters; the maximum is 200", len(row.Title)),
			})
		}

		if row.Status == "" {
			row.Status = StatusTodo
		} else {
			// Spreadsheets love "In Progress"; the API means in_progress.
			// Normalising here rather than rejecting saves a round trip over
			// a difference nobody considers meaningful.
			row.Status = strings.ReplaceAll(strings.ToLower(row.Status), " ", "_")
			if !ValidStatus(row.Status) {
				rowErrs = append(rowErrs, RowError{
					Line: line, Column: "status",
					Message: fmt.Sprintf("%q is not a status; use todo, in_progress or done", row.Status),
				})
			}
		}

		if p := at(positionIdx); p != "" {
			pos, err := strconv.ParseFloat(p, 64)
			if err != nil {
				rowErrs = append(rowErrs, RowError{
					Line: line, Column: "position",
					Message: fmt.Sprintf("%q is not a number", p),
				})
			} else {
				row.Position = pos
			}
		}

		rows = append(rows, row)
	}

	if len(rows) == 0 && len(rowErrs) == 0 {
		return nil, nil, ErrNoRows
	}
	return rows, rowErrs, nil
}

// Import writes every row in one transaction, or writes none of them.
//
// Position: a row that did not carry one is placed after the project's current
// last card, in file order, spaced by 1000 the way the board's own inserts are
// — so an imported column keeps the CSV's ordering and a later drag between
// two rows still has room to land between them without renumbering.
//
// The audit entry records the import as ONE event against the project rather
// than one per task. A hundred rows should read as "somebody imported a
// hundred tasks", not bury the log under a hundred near-identical lines.
func (s *Service) Import(ctx context.Context, orgID, projectID, actorID string, rows []ImportRow) ([]Task, error) {
	ctx, span := otel.Tracer("beacon/tasks").Start(ctx, "tasks.Import",
		trace.WithAttributes(
			attribute.String("org_id", orgID),
			attribute.String("project_id", projectID),
			// A count is safe to record; the titles are not — they are
			// whatever a person typed. Same rule as Create's span.
			attribute.Int("row_count", len(rows)),
		),
	)
	defer span.End()

	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	if len(rows) > MaxImportRows {
		return nil, ErrTooManyRows
	}
	for i := range rows {
		if !ValidStatus(rows[i].Status) {
			span.RecordError(ErrInvalidStatus)
			return nil, ErrInvalidStatus
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("tasks.Import: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit

	qtx := s.repo.q.WithTx(tx)

	next, err := s.repo.nextPosition(ctx, qtx, orgID, projectID)
	if err != nil {
		return nil, fmt.Errorf("tasks.Import: %w", err)
	}

	created := make([]Task, 0, len(rows))
	for i, row := range rows {
		pos := row.Position
		if pos == 0 {
			pos = next + float64(i+1)*positionGap
		}
		t, err := s.repo.createTx(ctx, qtx, orgID, projectID, row.Title, row.Status, pos)
		if err != nil {
			// The row index is worth having in the error: without it a
			// constraint violation on row 73 reads as a mystery.
			return nil, fmt.Errorf("tasks.Import: row %d: %w", i+1, err)
		}
		created = append(created, t)
	}

	if err := audit.Write(ctx, qtx, audit.Entry{
		OrgID: orgID, ActorID: actorID,
		Action: "task.imported", ResourceType: "project", ResourceID: projectID,
	}); err != nil {
		return nil, fmt.Errorf("tasks.Import: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("tasks.Import: commit: %w", err)
	}
	return created, nil
}

// positionGap matches the spacing the board leaves between cards, so an
// imported column behaves exactly like a hand-built one.
const positionGap = 1000

// uuidParse keeps the two transactional helpers below from repeating the same
// parse-and-wrap block four times. Repo's older methods each spell it out
// inline; this is not a refactor of those, just a way to stop adding more.
func uuidParse(s, what string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse %s id: %w", what, err)
	}
	return id, nil
}

// createTx is Create against a caller-supplied Queries, so an import can run
// every insert inside one transaction. Repo.Create stays as it is: the single
// -task path has no transaction to join.
func (r *Repo) createTx(ctx context.Context, q *db.Queries, orgID, projectID, title, status string, position float64) (Task, error) {
	oid, err := uuidParse(orgID, "org")
	if err != nil {
		return Task{}, err
	}
	pid, err := uuidParse(projectID, "project")
	if err != nil {
		return Task{}, err
	}
	t, err := q.CreateTask(ctx, db.CreateTaskParams{
		OrgID:     oid,
		ProjectID: pid,
		Title:     title,
		Status:    status,
		Position:  position,
	})
	if err != nil {
		// Same mapping as Repo.Create: no rows means the project is not this
		// org's. In an import that aborts the whole transaction, which is the
		// intended outcome — a batch aimed at the wrong project should write
		// nothing rather than partly land.
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, ErrProjectNotFound
		}
		return Task{}, fmt.Errorf("create: %w", err)
	}
	return toTask(t), nil
}

// nextPosition returns the highest position currently in the project, or 0 for
// an empty one. Read inside the import's own transaction so a concurrent
// import cannot read the same last position and interleave with this one.
func (r *Repo) nextPosition(ctx context.Context, q *db.Queries, orgID, projectID string) (float64, error) {
	oid, err := uuidParse(orgID, "org")
	if err != nil {
		return 0, err
	}
	pid, err := uuidParse(projectID, "project")
	if err != nil {
		return 0, err
	}
	pos, err := q.MaxTaskPosition(ctx, db.MaxTaskPositionParams{OrgID: oid, ProjectID: pid})
	if err != nil {
		return 0, fmt.Errorf("max position: %w", err)
	}
	return pos, nil
}

package http

// Tasks, comments and attachments.
//
// Two details in here are worth more attention than their line count suggests.
//
// PATCH on a task is a full replacement of the mutable fields — title AND
// status are both required. Moving a card on the board is this call with a new
// status and position, which means a client that sends only the status blanks
// the title. Declaring both as required puts that in the contract, so the
// generated SDK will not let a caller make that mistake silently.
//
// Attachments never carry bytes through this API. Creating one reserves a row
// and returns a presigned URL; the client PUTs the file straight to storage.
// That is why the create response has an upload_url and the read response has
// a download_url, and why either can be absent when storage is switched off.

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/attachments"
	"beacon/internal/auth"
	"beacon/internal/tasks"
)

type ListTasksInput struct {
	ProjectPath
	Paging
	Status string `query:"status" enum:"todo,in_progress,done" doc:"Optional column filter"`
}

type ListTasksOutput struct {
	Body ListBody[tasks.Task]
}

type CreateTaskInput struct {
	ProjectPath
	IdempotencyHeader
	Body struct {
		Title  string `json:"title" minLength:"1" maxLength:"200" required:"true"`
		Status string `json:"status,omitempty" enum:"todo,in_progress,done" doc:"Defaults to todo"`
		// A float so a card can be inserted between two others without
		// renumbering the column.
		Position float64 `json:"position,omitempty"`
	}
}

// ImportTasksInput carries the CSV as a string rather than a multipart upload.
//
// This API's contract is generated from these structs, and a typed string
// field survives that trip intact — the SDK gets `csv: string` and the web
// client reads the file and sends its text. A multipart endpoint would be the
// one operation the generated client could not describe, which is a high price
// for saving the caller a FileReader.
type ImportTasksInput struct {
	ProjectPath
	IdempotencyHeader
	Body struct {
		CSV string `json:"csv" minLength:"1" required:"true" doc:"CSV text with a title column, optionally status and position"`
	}
}

// ImportTasksOutput reports what landed. There is no partial-success shape
// here on purpose: the import is all-or-nothing, so either every row is in
// Tasks or the call failed and this body was never written.
//
// Row-level failures come back through the ordinary error envelope, in its
// rows field — not a second response shape. humaapi.go explains why this API
// has exactly one error shape; an import is not special enough to be the
// exception a client has to special-case.
type ImportTasksOutput struct {
	Status int
	Body   struct {
		Imported int          `json:"imported"`
		Tasks    []tasks.Task `json:"tasks"`
	}
}

type TaskInput struct{ TaskPath }

type UpdateTaskInput struct {
	TaskPath
	IdempotencyHeader
	Body struct {
		// Both required: this is a replacement, not a merge. Sending only the
		// status would blank the title.
		Title    string  `json:"title" minLength:"1" maxLength:"200" required:"true"`
		Status   string  `json:"status" enum:"todo,in_progress,done" required:"true"`
		Position float64 `json:"position,omitempty"`
	}
}

type TaskOutput struct {
	Status int
	Body   tasks.Task
}

type ListCommentsInput struct {
	TaskPath
	Paging
}

type ListCommentsOutput struct {
	Body ListBody[tasks.Comment]
}

type CreateCommentInput struct {
	TaskPath
	IdempotencyHeader
	Body struct {
		Body string `json:"body" minLength:"1" maxLength:"10000" required:"true"`
	}
}

type UpdateCommentInput struct {
	CommentPath
	IdempotencyHeader
	Body struct {
		Body string `json:"body" minLength:"1" maxLength:"10000" required:"true"`
	}
}

type DeleteCommentInput struct{ CommentPath }

type CommentOutput struct {
	Status int
	Body   tasks.Comment
}

type ListAttachmentsInput struct {
	TaskPath
	Paging
}

type ListAttachmentsOutput struct {
	Body ListBody[attachments.Attachment]
}

type CreateAttachmentInput struct {
	TaskPath
	IdempotencyHeader
	Body struct {
		Filename    string `json:"filename" minLength:"1" maxLength:"500" required:"true"`
		ContentType string `json:"content_type" minLength:"1" maxLength:"200" required:"true"`
		Size        int64  `json:"size" minimum:"1" maximum:"5368709120" required:"true" doc:"Bytes, up to 5 GiB"`
	}
}

type AttachmentInput struct {
	TaskPath
	AttachmentID string `path:"attachmentID" format:"uuid"`
}

// AttachmentEnvelope pairs the stored row with whichever presigned URL applies:
// upload on create, download on read. Both are short-lived, and both are absent
// when storage is not configured.
type AttachmentEnvelope struct {
	Attachment  attachments.Attachment `json:"attachment"`
	UploadURL   string                 `json:"upload_url,omitempty" doc:"Presigned PUT. Create only."`
	DownloadURL string                 `json:"download_url,omitempty" doc:"Presigned GET. Read only."`
}

type AttachmentOutput struct {
	Status int
	Body   AttachmentEnvelope
}

func (s *Server) registerTasks(api huma.API, g gates) {
	sec := []map[string][]string{{"bearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "list-tasks",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks",
		Summary:     "Tasks in the project",
		Tags:        []string{"tasks"},
		Security:    sec,
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *ListTasksInput) (*ListTasksOutput, error) {
		list, err := s.tasks.List(ctx, in.OrgID, in.ProjectID, in.Status, in.Limit, in.Offset)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ListTasksOutput{Body: listBody(list, in.Limit, in.Offset)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-task",
		Method:        http.MethodPost,
		Path:          "/v1/orgs/{orgID}/projects/{projectID}/tasks",
		Summary:       "Create a task",
		Tags:          []string{"tasks"},
		Security:      sec,
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.orgScoped,
	}, func(ctx context.Context, in *CreateTaskInput) (*TaskOutput, error) {
		t, err := s.tasks.Create(ctx, in.OrgID, in.ProjectID,
			in.Body.Title, in.Body.Status, in.Body.Position)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		s.notifyTaskEvent(ctx, in.OrgID, eventTaskCreated, t)
		return &TaskOutput{Status: http.StatusCreated, Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "import-tasks",
		Method:      http.MethodPost,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks/import",
		Summary:     "Import tasks from CSV",
		Description: "Creates many tasks in one transaction from CSV text. " +
			"The CSV needs a title column; status and position are optional. " +
			"If any row is invalid nothing is written and the response lists " +
			"every bad row, so one upload reports every problem at once.",
		Tags:          []string{"tasks"},
		Security:      sec,
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.orgScoped,
	}, func(ctx context.Context, in *ImportTasksInput) (*ImportTasksOutput, error) {
		rows, rowErrs, err := tasks.ParseCSV(strings.NewReader(in.Body.CSV))
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		if len(rowErrs) > 0 {
			return nil, s.importRowError(rowErrs)
		}

		userID, _ := auth.UserIDFrom(ctx)
		created, err := s.tasks.Import(ctx, in.OrgID, in.ProjectID, userID, rows)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}

		// One event per task, same as if they had been created one at a time:
		// a board open in another tab should end up in the same state either
		// way, and the client already knows how to apply this event.
		for _, t := range created {
			s.notifyTaskEvent(ctx, in.OrgID, eventTaskCreated, t)
		}

		out := &ImportTasksOutput{Status: http.StatusCreated}
		out.Body.Imported = len(created)
		out.Body.Tasks = created
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-task",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}",
		Summary:     "One task",
		Tags:        []string{"tasks"},
		Security:    sec,
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *TaskInput) (*TaskOutput, error) {
		t, err := s.tasks.Get(ctx, in.OrgID, in.TaskID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &TaskOutput{Status: http.StatusOK, Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-task",
		Method:      http.MethodPatch,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}",
		Summary:     "Update a task",
		Description: "A full replacement of the mutable fields. Moving a card on " +
			"the board is this call with a new status and position.",
		Tags:        []string{"tasks"},
		Security:    sec,
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *UpdateTaskInput) (*TaskOutput, error) {
		t, err := s.tasks.Update(ctx, in.OrgID, in.TaskID,
			in.Body.Title, in.Body.Status, in.Body.Position)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		s.notifyTaskEvent(ctx, in.OrgID, eventTaskUpdated, t)
		return &TaskOutput{Status: http.StatusOK, Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-task",
		Method:        http.MethodDelete,
		Path:          "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}",
		Summary:       "Delete a task",
		Tags:          []string{"tasks"},
		Security:      sec,
		DefaultStatus: http.StatusNoContent,
		Middlewares:   g.orgScoped,
	}, func(ctx context.Context, in *TaskInput) (*NoContentOutput, error) {
		userID, _ := auth.UserIDFrom(ctx)
		if err := s.tasks.Delete(ctx, in.OrgID, userID, in.TaskID); err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		s.notifyTaskEvent(ctx, in.OrgID, eventTaskDeleted, map[string]string{"id": in.TaskID})
		return &NoContentOutput{}, nil
	})

	// ---- comments ----------------------------------------------------------

	huma.Register(api, huma.Operation{
		OperationID: "list-comments",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}/comments",
		Summary:     "Comments on the task",
		Tags:        []string{"comments"},
		Security:    sec,
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *ListCommentsInput) (*ListCommentsOutput, error) {
		list, err := s.tasks.Comments(ctx, in.OrgID, in.TaskID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ListCommentsOutput{Body: page(list, in.Limit, in.Offset)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-comment",
		Method:        http.MethodPost,
		Path:          "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}/comments",
		Summary:       "Comment on the task",
		Tags:          []string{"comments"},
		Security:      sec,
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.orgScoped,
	}, func(ctx context.Context, in *CreateCommentInput) (*CommentOutput, error) {
		authorID, _ := auth.UserIDFrom(ctx)
		c, err := s.tasks.AddComment(ctx, in.OrgID, in.TaskID, authorID, in.Body.Body)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &CommentOutput{Status: http.StatusCreated, Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-comment",
		Method:      http.MethodPatch,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}/comments/{commentID}",
		Summary:     "Edit a comment",
		Description: "Only the comment's author may edit it. The response carries " +
			"`edited: true` once the body has changed, so a reader can see that " +
			"what they are reading is not what was first posted.",
		Tags:        []string{"comments"},
		Security:    sec,
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *UpdateCommentInput) (*CommentOutput, error) {
		actorID, _ := auth.UserIDFrom(ctx)
		c, err := s.tasks.UpdateComment(ctx, in.OrgID, in.CommentID, actorID, in.Body.Body)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &CommentOutput{Status: http.StatusOK, Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-comment",
		Method:      http.MethodDelete,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}/comments/{commentID}",
		Summary:     "Delete a comment",
		Description: "The author may delete their own comment; an org admin or " +
			"owner may delete anyone's, so a leaked secret can be removed by " +
			"someone other than whoever posted it.",
		Tags:          []string{"comments"},
		Security:      sec,
		DefaultStatus: http.StatusNoContent,
		Middlewares:   g.orgScoped,
	}, func(ctx context.Context, in *DeleteCommentInput) (*NoContentOutput, error) {
		actorID, _ := auth.UserIDFrom(ctx)
		// The role was put on the context by humaRequireOrg, which has already
		// proved membership. Reading it here rather than re-querying keeps the
		// moderation check on the same fact the gate used.
		actorRole, _ := auth.RoleFrom(ctx)
		if err := s.tasks.DeleteComment(ctx, in.OrgID, in.CommentID, actorID, actorRole); err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &NoContentOutput{}, nil
	})

	// ---- attachments -------------------------------------------------------

	huma.Register(api, huma.Operation{
		OperationID: "list-attachments",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}/attachments",
		Summary:     "Attachments on the task",
		Tags:        []string{"attachments"},
		Security:    sec,
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *ListAttachmentsInput) (*ListAttachmentsOutput, error) {
		if s.storage == nil {
			return nil, storageDisabledError()
		}
		list, err := s.attachments.List(ctx, in.OrgID, in.TaskID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ListAttachmentsOutput{Body: page(list, in.Limit, in.Offset)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-attachment",
		Method:        http.MethodPost,
		Path:          "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}/attachments",
		Summary:       "Reserve an attachment and get a presigned upload URL",
		Description:   "The file never passes through this API; PUT the bytes to upload_url.",
		Tags:          []string{"attachments"},
		Security:      sec,
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.orgScoped,
	}, func(ctx context.Context, in *CreateAttachmentInput) (*AttachmentOutput, error) {
		if s.storage == nil {
			return nil, storageDisabledError()
		}
		a, err := s.attachments.Create(ctx, in.OrgID, in.TaskID,
			in.Body.Filename, in.Body.ContentType, in.Body.Size)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		url, err := s.storage.PresignUpload(ctx, a.StorageKey, a.ContentType)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &AttachmentOutput{
			Status: http.StatusCreated,
			Body:   AttachmentEnvelope{Attachment: a, UploadURL: url},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-attachment",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/projects/{projectID}/tasks/{taskID}/attachments/{attachmentID}",
		Summary:     "One attachment, with a presigned download URL",
		Tags:        []string{"attachments"},
		Security:    sec,
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *AttachmentInput) (*AttachmentOutput, error) {
		if s.storage == nil {
			return nil, storageDisabledError()
		}
		a, err := s.attachments.Get(ctx, in.OrgID, in.AttachmentID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		url, err := s.storage.PresignDownload(ctx, a.StorageKey)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &AttachmentOutput{
			Status: http.StatusOK,
			Body:   AttachmentEnvelope{Attachment: a, DownloadURL: url},
		}, nil
	})
}

// storageDisabledError is the 501 a Beacon with no object storage answers.
// It is a configuration fact, not a failure, so it says so plainly.
func storageDisabledError() error {
	return &humaError{
		status: http.StatusNotImplemented,
		Body: errorBody{
			Code:    "storage_disabled",
			Message: "file storage is not configured on this server",
		},
	}
}

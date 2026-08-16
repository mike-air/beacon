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
		Status string `json:"status" enum:"todo,in_progress,done" doc:"Defaults to todo"`
		// A float so a card can be inserted between two others without
		// renumbering the column.
		Position float64 `json:"position"`
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
		Position float64 `json:"position"`
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
		return &ListTasksOutput{Body: ListBody[tasks.Task]{
			Items: list, Limit: in.Limit, Offset: in.Offset,
		}}, nil
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
		return &TaskOutput{Body: t}, nil
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
		return &TaskOutput{Body: t}, nil
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
		if err := s.tasks.Delete(ctx, in.OrgID, in.TaskID); err != nil {
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
		return &AttachmentOutput{Body: AttachmentEnvelope{Attachment: a, DownloadURL: url}}, nil
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

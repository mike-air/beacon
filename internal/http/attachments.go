package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/storage"
)

// Attachment handlers, under .../tasks/{taskID}/attachments. The file bytes
// never pass through us: POST creates the metadata row and returns a presigned
// upload URL; GET /{id} returns a presigned download URL. All org-scoped.
//
// Course mapping: Chapter 22 — file uploads with presigned S3 URLs. When storage
// is unconfigured these endpoints answer 501 (see storageEnabledOr501) so the
// service still boots with only Postgres.

type createAttachmentRequest struct {
	Filename    string `json:"filename"     validate:"required,min=1,max=500"`
	ContentType string `json:"content_type" validate:"required,min=1,max=200"`
	Size        int64  `json:"size"         validate:"required,min=1,max=5368709120"` // ≤ 5 GiB
}

// attachmentResponse pairs the stored metadata with a presigned URL.
type attachmentResponse struct {
	Attachment  any    `json:"attachment"`
	UploadURL   string `json:"upload_url,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

// storageEnabledOr501 returns the configured Storage, or writes a clean 501 and
// returns nil when storage is disabled.
func (s *Server) storageEnabledOr501(w http.ResponseWriter) storage.Storage {
	if s.storage == nil {
		writeError(w, http.StatusNotImplemented, "storage_disabled", "file storage is not configured on this server")
		return nil
	}
	return s.storage
}

// handleCreateAttachment records an attachment and returns a presigned upload
// URL. POST .../tasks/{taskID}/attachments.
func (s *Server) handleCreateAttachment(w http.ResponseWriter, r *http.Request) {
	store := s.storageEnabledOr501(w)
	if store == nil {
		return
	}
	orgID := chi.URLParam(r, "orgID")
	taskID := chi.URLParam(r, "taskID")

	var req createAttachmentRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	att, err := s.attachments.Create(r.Context(), orgID, taskID, req.Filename, req.ContentType, req.Size)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	uploadURL, err := store.PresignUpload(r.Context(), att.StorageKey, att.ContentType)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, attachmentResponse{Attachment: att, UploadURL: uploadURL})
}

// handleListAttachments lists a task's attachments. GET .../tasks/{taskID}/attachments.
func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	if s.storageEnabledOr501(w) == nil {
		return
	}
	orgID := chi.URLParam(r, "orgID")
	taskID := chi.URLParam(r, "taskID")
	list, err := s.attachments.List(r.Context(), orgID, taskID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeList(w, r, list)
}

// handleGetAttachment returns a presigned download URL for one attachment.
// GET .../attachments/{attachmentID}.
func (s *Server) handleGetAttachment(w http.ResponseWriter, r *http.Request) {
	store := s.storageEnabledOr501(w)
	if store == nil {
		return
	}
	orgID := chi.URLParam(r, "orgID")
	id := chi.URLParam(r, "attachmentID")

	att, err := s.attachments.Get(r.Context(), orgID, id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	downloadURL, err := store.PresignDownload(r.Context(), att.StorageKey)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, attachmentResponse{Attachment: att, DownloadURL: downloadURL})
}

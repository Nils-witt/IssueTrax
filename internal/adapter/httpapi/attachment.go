package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

const (
	// uploadFormField is the multipart form field that carries the file.
	uploadFormField = "file"
	// maxMultipartOverhead is what a multipart body may add to the file:
	// boundaries, part headers and other (ignored) small fields.
	maxMultipartOverhead = 64 << 10
	// transferTimeout replaces the server's read or write timeout while a
	// file is uploaded or downloaded, so slow connections can finish.
	transferTimeout = 5 * time.Minute
	// attachmentCSP applies to served files: nothing in them may run or
	// load anything, even when a browser opens one directly.
	attachmentCSP = "default-src 'none'; sandbox"
)

// inlineMediaTypes are the (sniffed) types browsers may show in the page;
// every other file is served as a download.
var inlineMediaTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

// errBadUpload marks request bodies that are not a multipart form.
var errBadUpload = errors.New("invalid multipart body")

type attachmentResponse struct {
	ID        string       `json:"id"`
	FileName  string       `json:"fileName"`
	MediaType string       `json:"mediaType"`
	Size      int64        `json:"size"`
	Uploader  userResponse `json:"uploader"`
	CreatedAt time.Time    `json:"createdAt"`
}

func toAttachmentResponse(v port.AttachmentView) attachmentResponse {
	return attachmentResponse{
		ID:        v.Attachment.ID.String(),
		FileName:  v.Attachment.FileName,
		MediaType: v.Attachment.MediaType,
		Size:      v.Attachment.Size,
		Uploader:  toUserResponse(v.Uploader),
		CreatedAt: v.Attachment.CreatedAt,
	}
}

// GET /api/projects/{key}/issues/{number}/attachments.
func (h *Handler) listAttachments(w http.ResponseWriter, r *http.Request, _ domain.User) {
	listOfIssue(h, w, r, h.attachments.List, toAttachmentResponse)
}

// POST /api/projects/{key}/issues/{number}/attachments (multipart/form-data
// with the file in the "file" field).
func (h *Handler) uploadAttachment(w http.ResponseWriter, r *http.Request, user domain.User) {
	number, ok := issueNumber(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrIssueNotFound)
		return
	}

	// Not every ResponseWriter supports deadlines; then the server's apply.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(transferTimeout))

	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxAttachmentBytes+maxMultipartOverhead)

	in, err := readUpload(r)
	if errors.Is(err, errBadUpload) {
		writeError(w, http.StatusBadRequest, codeInvalidUpload, "request body is not a multipart form with a file")
		return
	}

	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	view, err := h.attachments.Upload(r.Context(), user.ID, projectKey(r), number, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toAttachmentResponse(view))
}

// readUpload reads the first "file" part of a multipart body, skipping
// other parts. It returns a validation error when there is no file or the
// body is too large, and errBadUpload when the body is not multipart.
func readUpload(r *http.Request) (port.UploadAttachmentInput, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return port.UploadAttachmentInput{}, fmt.Errorf("%w: %w", errBadUpload, err)
	}

	for {
		part, err := mr.NextPart()

		switch {
		case errors.Is(err, io.EOF):
			return port.UploadAttachmentInput{}, domain.NewValidationError(map[string]string{"file": domain.FieldRequired})
		case err != nil:
			return port.UploadAttachmentInput{}, uploadReadError(err)
		}

		if part.FormName() != uploadFormField {
			continue
		}

		// One byte more than allowed is enough to tell the file is too
		// large; the domain then rejects it.
		content, err := io.ReadAll(io.LimitReader(part, domain.MaxAttachmentBytes+1))
		if err != nil {
			return port.UploadAttachmentInput{}, uploadReadError(err)
		}

		return port.UploadAttachmentInput{
			FileName:  part.FileName(),
			MediaType: part.Header.Get("Content-Type"),
			Content:   content,
		}, nil
	}
}

// uploadReadError classifies an error while reading the multipart body.
func uploadReadError(err error) error {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return domain.NewValidationError(map[string]string{"file": domain.FieldTooLong})
	}

	return fmt.Errorf("%w: %w", errBadUpload, err)
}

// GET /api/projects/{key}/issues/{number}/attachments/{attachmentId} serves
// the file. Safe images are shown inline; everything else is a download.
// Range and conditional requests are supported.
func (h *Handler) downloadAttachment(w http.ResponseWriter, r *http.Request, _ domain.User) {
	number, id, ok := h.attachmentPath(w, r)
	if !ok {
		return
	}

	attachment, content, err := h.attachments.Content(r.Context(), projectKey(r), number, id)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(transferTimeout))

	mediaType, disposition := attachment.MediaType, "attachment"
	if sniffed := http.DetectContentType(content); inlineMediaTypes[sniffed] {
		mediaType, disposition = sniffed, "inline"
	}

	if d := mime.FormatMediaType(disposition, map[string]string{"filename": attachment.FileName}); d != "" {
		disposition = d
	}

	hdr := w.Header()
	hdr.Set("Content-Type", mediaType)
	hdr.Set("Content-Disposition", disposition)
	hdr.Set("Content-Security-Policy", attachmentCSP)
	// Content never changes; private keeps shared caches from storing it.
	hdr.Set("Cache-Control", "private, max-age=3600")
	hdr.Set("ETag", `"`+attachment.ID.String()+`"`)

	http.ServeContent(w, r, "", attachment.CreatedAt, bytes.NewReader(content))
}

// DELETE /api/projects/{key}/issues/{number}/attachments/{attachmentId}.
func (h *Handler) deleteAttachment(w http.ResponseWriter, r *http.Request, user domain.User) {
	number, id, ok := h.attachmentPath(w, r)
	if !ok {
		return
	}

	if err := h.attachments.Delete(r.Context(), user.ID, projectKey(r), number, id); err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// attachmentPath parses {number} and {attachmentId}, answering 404 when
// either is malformed.
func (h *Handler) attachmentPath(w http.ResponseWriter, r *http.Request) (int64, domain.AttachmentID, bool) {
	number, ok := issueNumber(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrIssueNotFound)
		return 0, domain.AttachmentID{}, false
	}

	id, err := uuid.Parse(r.PathValue("attachmentId"))
	if err != nil {
		h.writeDomainError(w, r, domain.ErrAttachmentNotFound)
		return 0, domain.AttachmentID{}, false
	}

	return number, domain.AttachmentID(id), true
}

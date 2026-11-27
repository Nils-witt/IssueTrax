package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type issueResponse struct {
	ID          string         `json:"id"`
	Key         string         `json:"key"`
	Number      int64          `json:"number"`
	ProjectKey  string         `json:"projectKey"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Tags        []tagResponse  `json:"tags"`
	Assignees   []userResponse `json:"assignees"`
	// StorageItems are the linked storage items, deleted ones included.
	StorageItems []storageItemRefResponse `json:"storageItems"`
	CreatedBy    userResponse             `json:"createdBy"`
	CreatedAt    time.Time                `json:"createdAt"`
	UpdatedAt    time.Time                `json:"updatedAt"`
}

func toIssueResponse(v port.IssueView) issueResponse {
	resp := issueResponse{
		ID:           v.Issue.ID.String(),
		Key:          v.Issue.Key(v.ProjectKey),
		Number:       v.Issue.Number,
		ProjectKey:   string(v.ProjectKey),
		Title:        v.Issue.Title,
		Description:  v.Issue.Description,
		Status:       string(v.Issue.Status),
		Tags:         make([]tagResponse, 0, len(v.Tags)),
		Assignees:    make([]userResponse, 0, len(v.Assignees)),
		StorageItems: make([]storageItemRefResponse, 0, len(v.StorageItems)),
		CreatedBy:    toUserResponse(v.CreatedBy),
		CreatedAt:    v.Issue.CreatedAt,
		UpdatedAt:    v.Issue.UpdatedAt,
	}

	for _, t := range v.Tags {
		resp.Tags = append(resp.Tags, toTagResponse(t))
	}

	for _, u := range v.Assignees {
		resp.Assignees = append(resp.Assignees, toUserResponse(u))
	}

	for _, i := range v.StorageItems {
		resp.StorageItems = append(resp.StorageItems, toStorageItemRefResponse(i))
	}

	return resp
}

type createIssueRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	Assignees   []string `json:"assignees"`
	// StorageItems are IDs of storage items of any kind to link.
	StorageItems []string `json:"storageItems"`
}

// updateIssueRequest is a partial update: absent (or null) fields stay
// unchanged; tags, assignees and storage items replace the whole set when
// present.
type updateIssueRequest struct {
	Title        *string   `json:"title"`
	Description  *string   `json:"description"`
	Status       *string   `json:"status"`
	Tags         *[]string `json:"tags"`
	Assignees    *[]string `json:"assignees"`
	StorageItems *[]string `json:"storageItems"`
}

// GET /api/projects/{key}/issues?status=&tag=&tagMode=&assignee=&assigneeMode=&storageItem=&storageItemMode=&q=&limit=&cursor=.
func (h *Handler) listIssues(w http.ResponseWriter, r *http.Request, _ domain.User) {
	q := r.URL.Query()

	page, err := pageRequest(q)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	filter, err := issueFilter(q)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	issues, err := h.issues.List(r.Context(), projectKey(r), filter, page)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toPageResponse(issues, toIssueResponse))
}

func issueFilter(q map[string][]string) (port.IssueFilter, error) {
	tags, err := parseIDs[domain.TagID](q["tag"], "tag")
	if err != nil {
		return port.IssueFilter{}, err
	}

	assignees, err := parseIDs[domain.UserID](q["assignee"], "assignee")
	if err != nil {
		return port.IssueFilter{}, err
	}

	items, err := parseIDs[domain.StorageItemID](q["storageItem"], "storageItem")
	if err != nil {
		return port.IssueFilter{}, err
	}

	statuses := make([]domain.IssueStatus, 0, len(q["status"]))
	for _, s := range q["status"] {
		statuses = append(statuses, domain.IssueStatus(s))
	}

	first := func(key string) string {
		if v := q[key]; len(v) > 0 {
			return v[0]
		}

		return ""
	}

	return port.IssueFilter{
		Statuses:        statuses,
		TagIDs:          tags,
		TagMode:         port.MatchMode(first("tagMode")),
		AssigneeIDs:     assignees,
		AssigneeMode:    port.MatchMode(first("assigneeMode")),
		StorageItemIDs:  items,
		StorageItemMode: port.MatchMode(first("storageItemMode")),
		Query:           first("q"),
	}, nil
}

// POST /api/projects/{key}/issues.
func (h *Handler) createIssue(w http.ResponseWriter, r *http.Request, user domain.User) {
	var req createIssueRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	assignees, err := parseIDs[domain.UserID](req.Assignees, "assignees")
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	items, err := parseIDs[domain.StorageItemID](req.StorageItems, "storageItems")
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	view, err := h.issues.Create(r.Context(), user.ID, projectKey(r), port.CreateIssueInput{
		Title:        req.Title,
		Description:  req.Description,
		Status:       domain.IssueStatus(req.Status),
		Tags:         req.Tags,
		Assignees:    assignees,
		StorageItems: items,
	})
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, toIssueResponse(view))
}

// GET /api/projects/{key}/issues/{number}.
func (h *Handler) getIssue(w http.ResponseWriter, r *http.Request, _ domain.User) {
	number, ok := issueNumber(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrIssueNotFound)
		return
	}

	view, err := h.issues.Get(r.Context(), projectKey(r), number)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toIssueResponse(view))
}

// PATCH /api/projects/{key}/issues/{number}.
func (h *Handler) updateIssue(w http.ResponseWriter, r *http.Request, user domain.User) {
	number, ok := issueNumber(r)
	if !ok {
		h.writeDomainError(w, r, domain.ErrIssueNotFound)
		return
	}

	var req updateIssueRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	in := port.UpdateIssueInput{Title: req.Title, Description: req.Description, Tags: req.Tags}

	if req.Status != nil {
		in.Status = new(domain.IssueStatus(*req.Status))
	}

	if req.Assignees != nil {
		assignees, err := parseIDs[domain.UserID](*req.Assignees, "assignees")
		if err != nil {
			h.writeDomainError(w, r, err)
			return
		}

		in.Assignees = &assignees
	}

	if req.StorageItems != nil {
		items, err := parseIDs[domain.StorageItemID](*req.StorageItems, "storageItems")
		if err != nil {
			h.writeDomainError(w, r, err)
			return
		}

		in.StorageItems = &items
	}

	view, err := h.issues.Update(r.Context(), user.ID, projectKey(r), number, in)
	if err != nil {
		h.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toIssueResponse(view))
}

// issueNumber parses {number}; anything but a positive integer is not found.
func issueNumber(r *http.Request) (int64, bool) {
	n, err := strconv.ParseInt(r.PathValue("number"), 10, 64)

	return n, err == nil && n > 0
}

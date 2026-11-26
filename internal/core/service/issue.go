package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// maxQueryRunes bounds the free-text search term.
const maxQueryRunes = 200

// Compile-time check that IssueService satisfies the driving port.
var _ port.IssueService = (*IssueService)(nil)

// IssueService implements port.IssueService.
type IssueService struct {
	repos Repositories
	opts  options
}

// NewIssueService returns an IssueService backed by repos.
func NewIssueService(repos Repositories, opts ...Option) *IssueService {
	return &IssueService{repos: repos, opts: newOptions(opts)}
}

// Create validates and stores a new issue, creating unknown tags on the fly.
func (s *IssueService) Create(
	ctx context.Context, actor domain.UserID, key domain.ProjectKey, in port.CreateIssueInput,
) (port.IssueView, error) {
	var view port.IssueView

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		project, err := projectByKey(ctx, s.repos.Projects, key)
		if err != nil {
			return err
		}

		tags, err := s.resolveTags(ctx, project.ID, in.Tags)
		if err != nil {
			return err
		}

		assignees, err := s.resolveAssignees(ctx, in.Assignees)
		if err != nil {
			return err
		}

		number, err := s.repos.Projects.AllocateIssueNumber(ctx, project.ID)
		if err != nil {
			return fmt.Errorf("allocate issue number in %q: %w", project.Key, err)
		}

		issue, err := domain.NewIssue(domain.NewIssueParams{
			ID:          domain.IssueID(s.opts.newID()),
			ProjectID:   project.ID,
			Number:      number,
			Title:       in.Title,
			Description: in.Description,
			Status:      in.Status,
			Assignees:   assignees,
			Tags:        tags,
			CreatedBy:   actor,
			Now:         s.opts.now(),
		})
		if err != nil {
			return err
		}

		if err := s.repos.Issues.Create(ctx, issue); err != nil {
			return fmt.Errorf("create issue %s: %w", issue.Key(project.Key), err)
		}

		if view, err = s.view(ctx, project, issue); err != nil {
			return err
		}

		changes := domain.AuditChanges{}
		changes.Initial("title", issue.Title)
		changes.Initial("status", string(issue.Status))
		changes.InitialText("description", issue.Description)
		changes.InitialList("tags", tagNames(view.Tags))
		changes.InitialList("assignees", userNames(view.Assignees))

		return record(ctx, s.repos, s.opts, issueEvent(actor, domain.AuditIssueCreate, project, issue, changes))
	})
	if err != nil {
		return port.IssueView{}, err
	}

	s.opts.notify(ctx, domain.Change{Kind: domain.ChangeIssue, Project: view.ProjectKey, Issue: view.Issue.Number})

	return view, nil
}

// Get returns the issue with the given project-scoped number.
func (s *IssueService) Get(ctx context.Context, key domain.ProjectKey, number int64) (port.IssueView, error) {
	project, err := projectByKey(ctx, s.repos.Projects, key)
	if err != nil {
		return port.IssueView{}, err
	}

	issue, err := s.repos.Issues.Get(ctx, project.ID, number)
	if err != nil {
		return port.IssueView{}, fmt.Errorf("get issue %s-%d: %w", project.Key, number, err)
	}

	return s.view(ctx, project, issue)
}

// List returns one page of matching issues, newest first.
func (s *IssueService) List(
	ctx context.Context, key domain.ProjectKey, filter port.IssueFilter, page domain.PageRequest,
) (domain.Page[port.IssueView], error) {
	filter, err := normalizeFilter(filter)
	if err != nil {
		return domain.Page[port.IssueView]{}, err
	}

	project, err := projectByKey(ctx, s.repos.Projects, key)
	if err != nil {
		return domain.Page[port.IssueView]{}, err
	}

	issues, err := s.repos.Issues.List(ctx, project.ID, filter, page)
	if err != nil {
		return domain.Page[port.IssueView]{}, fmt.Errorf("list issues in %q: %w", project.Key, err)
	}

	views, err := s.views(ctx, project, issues.Items)
	if err != nil {
		return domain.Page[port.IssueView]{}, err
	}

	return domain.Page[port.IssueView]{Items: views, Total: issues.Total, NextCursor: issues.NextCursor}, nil
}

// Update applies a partial update. Archived issues only accept a status change.
func (s *IssueService) Update(
	ctx context.Context, actor domain.UserID, key domain.ProjectKey, number int64, in port.UpdateIssueInput,
) (port.IssueView, error) {
	var view port.IssueView

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		project, err := projectByKey(ctx, s.repos.Projects, key)
		if err != nil {
			return err
		}

		issue, err := s.repos.Issues.Get(ctx, project.ID, number)
		if err != nil {
			return fmt.Errorf("get issue %s-%d: %w", project.Key, number, err)
		}

		changes, err := s.changes(ctx, issue, in)
		if err != nil {
			return err
		}

		before, err := s.view(ctx, project, issue)
		if err != nil {
			return err
		}

		if err := issue.Apply(changes, s.opts.now()); err != nil {
			return err
		}

		if err := s.repos.Issues.Update(ctx, issue); err != nil {
			return fmt.Errorf("update issue %s: %w", issue.Key(project.Key), err)
		}

		if view, err = s.view(ctx, project, issue); err != nil {
			return err
		}

		audit := issueDiff(before, view)
		if len(audit) == 0 {
			return nil
		}

		return record(ctx, s.repos, s.opts, issueEvent(actor, domain.AuditIssueUpdate, project, issue, audit))
	})
	if err != nil {
		return port.IssueView{}, err
	}

	s.opts.notify(ctx, domain.Change{Kind: domain.ChangeIssue, Project: view.ProjectKey, Issue: view.Issue.Number})

	return view, nil
}

// changes turns in into domain changes, resolving tags and assignees. The
// archived check runs first so an archived issue never creates tags.
func (s *IssueService) changes(ctx context.Context, issue domain.Issue, in port.UpdateIssueInput) (domain.IssueChanges, error) {
	changes := domain.IssueChanges{Title: in.Title, Description: in.Description, Status: in.Status}
	statusOnly := in.Title == nil && in.Description == nil && in.Tags == nil && in.Assignees == nil

	if err := issue.CheckEditable(statusOnly); err != nil {
		return domain.IssueChanges{}, err
	}

	if in.Tags != nil {
		tags, err := s.resolveTags(ctx, issue.ProjectID, *in.Tags)
		if err != nil {
			return domain.IssueChanges{}, err
		}

		changes.Tags = &tags
	}

	if in.Assignees != nil {
		assignees, err := s.resolveAssignees(ctx, *in.Assignees)
		if err != nil {
			return domain.IssueChanges{}, err
		}

		changes.Assignees = &assignees
	}

	return changes, nil
}

// resolveTags normalizes tag names and returns their IDs in input order,
// creating tags the project does not have yet.
func (s *IssueService) resolveTags(ctx context.Context, project domain.ProjectID, raw []string) ([]domain.TagID, error) {
	names := make([]domain.TagName, 0, len(raw))

	for _, r := range raw {
		name, ok := domain.NormalizeTagName(r)
		if !ok {
			return nil, domain.NewValidationError(map[string]string{"tags": domain.FieldInvalid})
		}

		names = append(names, name)
	}

	names = unique(names)
	if len(names) == 0 {
		return []domain.TagID{}, nil
	}

	existing, err := s.repos.Tags.FindByNames(ctx, project, names)
	if err != nil {
		return nil, fmt.Errorf("find tags: %w", err)
	}

	byName := make(map[domain.TagName]domain.TagID, len(existing))
	for _, t := range existing {
		byName[t.Name] = t.ID
	}

	ids := make([]domain.TagID, 0, len(names))

	for _, name := range names {
		id, ok := byName[name]
		if !ok {
			tag, err := s.repos.Tags.Ensure(ctx, domain.NewTag(domain.TagID(s.opts.newID()), project, name, s.opts.now()))
			if err != nil {
				return nil, fmt.Errorf("create tag %q: %w", name, err)
			}

			id = tag.ID
		}

		ids = append(ids, id)
	}

	return ids, nil
}

// resolveAssignees deduplicates ids and checks every user exists.
func (s *IssueService) resolveAssignees(ctx context.Context, ids []domain.UserID) ([]domain.UserID, error) {
	ids = unique(ids)
	if len(ids) == 0 {
		return ids, nil
	}

	users, err := s.repos.Users.GetMany(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get assignees: %w", err)
	}

	if len(users) != len(ids) {
		return nil, domain.NewValidationError(map[string]string{"assignees": domain.FieldUnknown})
	}

	return ids, nil
}

func (s *IssueService) view(ctx context.Context, project domain.Project, issue domain.Issue) (port.IssueView, error) {
	views, err := s.views(ctx, project, []domain.Issue{issue})
	if err != nil {
		return port.IssueView{}, err
	}

	return views[0], nil
}

// views resolves tags and users for issues with one lookup each.
func (s *IssueService) views(ctx context.Context, project domain.Project, issues []domain.Issue) ([]port.IssueView, error) {
	var (
		tagIDs  []domain.TagID
		userIDs []domain.UserID
	)

	for _, i := range issues {
		tagIDs = append(tagIDs, i.Tags...)
		userIDs = append(userIDs, i.Assignees...)
		userIDs = append(userIDs, i.CreatedBy)
	}

	tags, err := s.repos.Tags.GetMany(ctx, unique(tagIDs))
	if err != nil {
		return nil, fmt.Errorf("get issue tags: %w", err)
	}

	users, err := s.repos.Users.GetMany(ctx, unique(userIDs))
	if err != nil {
		return nil, fmt.Errorf("get issue users: %w", err)
	}

	tagByID := make(map[domain.TagID]domain.Tag, len(tags))
	for _, t := range tags {
		tagByID[t.ID] = t
	}

	userByID := make(map[domain.UserID]domain.User, len(users))
	for _, u := range users {
		userByID[u.ID] = u
	}

	user := func(id domain.UserID) domain.User {
		if u, ok := userByID[id]; ok {
			return u
		}

		return domain.User{ID: id}
	}

	views := make([]port.IssueView, 0, len(issues))

	for _, i := range issues {
		v := port.IssueView{
			Issue:      i,
			ProjectKey: project.Key,
			Tags:       make([]domain.Tag, 0, len(i.Tags)),
			Assignees:  make([]domain.User, 0, len(i.Assignees)),
			CreatedBy:  user(i.CreatedBy),
		}

		for _, id := range i.Tags {
			if t, ok := tagByID[id]; ok {
				v.Tags = append(v.Tags, t)
			}
		}

		for _, id := range i.Assignees {
			v.Assignees = append(v.Assignees, user(id))
		}

		// Present tags and assignees alphabetically, independent of storage order.
		slices.SortFunc(v.Tags, func(a, b domain.Tag) int { return cmp.Compare(a.Name, b.Name) })
		slices.SortFunc(v.Assignees, func(a, b domain.User) int {
			return cmp.Or(cmp.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName)), cmp.Compare(a.ID.String(), b.ID.String()))
		})

		views = append(views, v)
	}

	return views, nil
}

// normalizeFilter validates f and fills in defaults: statuses default to all
// but archived, match modes default to all.
func normalizeFilter(f port.IssueFilter) (port.IssueFilter, error) {
	fields := map[string]string{}

	for _, st := range f.Statuses {
		if !st.Valid() {
			fields["status"] = domain.FieldInvalid
		}
	}

	if len(f.Statuses) == 0 {
		f.Statuses = []domain.IssueStatus{domain.StatusDraft, domain.StatusOpen, domain.StatusClosed}
	}

	var ok bool

	if f.TagMode, ok = normalizeMode(f.TagMode); !ok {
		fields["tagMode"] = domain.FieldInvalid
	}

	if f.AssigneeMode, ok = normalizeMode(f.AssigneeMode); !ok {
		fields["assigneeMode"] = domain.FieldInvalid
	}

	f.Query = strings.TrimSpace(f.Query)
	if utf8.RuneCountInString(f.Query) > maxQueryRunes {
		fields["q"] = domain.FieldTooLong
	}

	if len(fields) > 0 {
		return port.IssueFilter{}, domain.NewValidationError(fields)
	}

	f.Statuses = unique(f.Statuses)
	f.TagIDs = unique(f.TagIDs)
	f.AssigneeIDs = unique(f.AssigneeIDs)

	return f, nil
}

func normalizeMode(m port.MatchMode) (port.MatchMode, bool) {
	switch m {
	case "", port.MatchAll:
		return port.MatchAll, true
	case port.MatchAny:
		return port.MatchAny, true
	default:
		return m, false
	}
}

// issueDiff lists the audited fields that differ between two views of an issue.
func issueDiff(before, after port.IssueView) domain.AuditChanges {
	changes := domain.AuditChanges{}
	changes.Set("title", before.Issue.Title, after.Issue.Title)
	changes.Set("status", string(before.Issue.Status), string(after.Issue.Status))
	changes.Text("description", before.Issue.Description, after.Issue.Description)
	changes.List("tags", tagNames(before.Tags), tagNames(after.Tags))
	changes.List("assignees", userNames(before.Assignees), userNames(after.Assignees))

	return changes
}

func issueEvent(
	actor domain.UserID, action domain.AuditAction, p domain.Project, i domain.Issue, changes domain.AuditChanges,
) auditEvent {
	return auditEvent{
		actor: &actor, action: action, changes: changes,
		targetType: domain.AuditTargetIssue, targetID: i.ID.String(), targetLabel: i.Key(p.Key),
	}
}

func tagNames(tags []domain.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, string(t.Name))
	}

	return names
}

func userNames(users []domain.User) []string {
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.DisplayName)
	}

	return names
}

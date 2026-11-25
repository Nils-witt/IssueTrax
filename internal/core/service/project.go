package service

import (
	"context"
	"fmt"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

// Compile-time check that ProjectService satisfies the driving port.
var _ port.ProjectService = (*ProjectService)(nil)

// ProjectService implements port.ProjectService.
type ProjectService struct {
	repos Repositories
	opts  options
}

// NewProjectService returns a ProjectService backed by repos.
func NewProjectService(repos Repositories, opts ...Option) *ProjectService {
	return &ProjectService{repos: repos, opts: newOptions(opts)}
}

// Create validates and stores a new project.
func (s *ProjectService) Create(ctx context.Context, in port.CreateProjectInput) (domain.Project, error) {
	project, err := domain.NewProject(domain.NewProjectParams{
		ID:          domain.ProjectID(s.opts.newID()),
		Key:         in.Key,
		Name:        in.Name,
		Description: in.Description,
		Now:         s.opts.now(),
	})
	if err != nil {
		return domain.Project{}, err
	}

	if err := s.repos.Projects.Create(ctx, project); err != nil {
		return domain.Project{}, fmt.Errorf("create project %q: %w", project.Key, err)
	}

	return project, nil
}

// Get returns the project with key.
func (s *ProjectService) Get(ctx context.Context, key domain.ProjectKey) (domain.Project, error) {
	return projectByKey(ctx, s.repos.Projects, key)
}

// List returns one page of projects ordered by name.
func (s *ProjectService) List(ctx context.Context, page domain.PageRequest) (domain.Page[domain.Project], error) {
	projects, err := s.repos.Projects.List(ctx, page)
	if err != nil {
		return domain.Page[domain.Project]{}, fmt.Errorf("list projects: %w", err)
	}

	return projects, nil
}

// Update changes a project's name and/or description.
func (s *ProjectService) Update(ctx context.Context, key domain.ProjectKey, in port.UpdateProjectInput) (domain.Project, error) {
	var project domain.Project

	err := s.repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error

		project, err = projectByKey(ctx, s.repos.Projects, key)
		if err != nil {
			return err
		}

		changes := domain.ProjectChanges{Name: in.Name, Description: in.Description}
		if err := project.Apply(changes, s.opts.now()); err != nil {
			return err
		}

		if err := s.repos.Projects.Update(ctx, project); err != nil {
			return fmt.Errorf("update project %q: %w", project.Key, err)
		}

		return nil
	})
	if err != nil {
		return domain.Project{}, err
	}

	return project, nil
}

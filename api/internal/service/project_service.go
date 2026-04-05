package service

import (
	"context"
	"github.com/magosproject/magos/api/v1alpha1"
	"log/slog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProjectService defines operations for Project resources.
type ProjectService interface {
	List(ctx context.Context) (*v1alpha1.ProjectList, error)
	Get(ctx context.Context, namespace, name string) (*v1alpha1.Project, error)
}

// projectService implements ProjectService.
type projectService struct {
	logger *slog.Logger
	client client.Client
}

// NewProjectService returns a new ProjectService.
func NewProjectService(logger *slog.Logger, k8sClient client.Client) ProjectService {
	return &projectService{logger: logger, client: k8sClient}
}

// List returns all Project resources across all namespaces.
func (s *projectService) List(ctx context.Context) (*v1alpha1.ProjectList, error) {
	s.logger.Info("listing Projects")
	var list v1alpha1.ProjectList
	if err := s.client.List(ctx, &list); err != nil {
		s.logger.Error("failed to list Projects", "error", err)
		return nil, err
	}
	s.logger.Info("Projects listed", "count", len(list.Items))
	return &list, nil
}

// Get returns a single Project resource by namespace and name.
func (s *projectService) Get(ctx context.Context, namespace, name string) (*v1alpha1.Project, error) {
	s.logger.Info("getting Project", "namespace", namespace, "name", name)
	var project v1alpha1.Project
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &project); err != nil {
		s.logger.Error("failed to get Project", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}
	s.logger.Info("Project retrieved", "namespace", namespace, "name", name)
	return &project, nil
}

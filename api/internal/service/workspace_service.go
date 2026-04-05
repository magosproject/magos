package service

import (
	"context"
	"github.com/magosproject/magos/api/v1alpha1"
	"log/slog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkspaceService defines operations for Workspace resources.
type WorkspaceService interface {
	List(ctx context.Context) (*v1alpha1.WorkspaceList, error)
	Get(ctx context.Context, namespace, name string) (*v1alpha1.Workspace, error)
}

type workspaceService struct {
	logger *slog.Logger
	client client.Client
}

func NewWorkspaceService(logger *slog.Logger, client client.Client) WorkspaceService {
	return &workspaceService{logger: logger, client: client}
}

func (s *workspaceService) List(ctx context.Context) (*v1alpha1.WorkspaceList, error) {
	s.logger.Info("listing Workspaces")
	var list v1alpha1.WorkspaceList
	if err := s.client.List(ctx, &list); err != nil {
		s.logger.Error("failed to list Workspaces", "error", err)
		return nil, err
	}
	s.logger.Info("Workspaces listed", "count", len(list.Items))
	return &list, nil
}

func (s *workspaceService) Get(ctx context.Context, namespace, name string) (*v1alpha1.Workspace, error) {
	s.logger.Info("getting Workspace", "namespace", namespace, "name", name)
	var obj v1alpha1.Workspace
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &obj); err != nil {
		s.logger.Error("failed to get Workspace", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}
	s.logger.Info("Workspace retrieved", "namespace", namespace, "name", name)
	return &obj, nil
}

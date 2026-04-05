package service

import (
	"context"
	"github.com/magosproject/magos/api/v1alpha1"
	"log/slog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RolloutService defines operations for Rollout resources.
type RolloutService interface {
	List(ctx context.Context) (*v1alpha1.RolloutList, error)
	Get(ctx context.Context, namespace, name string) (*v1alpha1.Rollout, error)
}

type rolloutService struct {
	logger *slog.Logger
	client client.Client
}

func NewRolloutService(logger *slog.Logger, client client.Client) RolloutService {
	return &rolloutService{logger: logger, client: client}
}

func (s *rolloutService) List(ctx context.Context) (*v1alpha1.RolloutList, error) {
	s.logger.Info("listing Rollouts")
	var list v1alpha1.RolloutList
	if err := s.client.List(ctx, &list); err != nil {
		s.logger.Error("failed to list Rollouts", "error", err)
		return nil, err
	}
	s.logger.Info("Rollouts listed", "count", len(list.Items))
	return &list, nil
}

func (s *rolloutService) Get(ctx context.Context, namespace, name string) (*v1alpha1.Rollout, error) {
	s.logger.Info("getting Rollout", "namespace", namespace, "name", name)
	var obj v1alpha1.Rollout
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &obj); err != nil {
		s.logger.Error("failed to get Rollout", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}
	s.logger.Info("Rollout retrieved", "namespace", namespace, "name", name)
	return &obj, nil
}

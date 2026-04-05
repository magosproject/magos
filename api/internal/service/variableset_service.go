package service

import (
	"context"
	"github.com/magosproject/magos/api/v1alpha1"
	"log/slog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VariableSetService defines operations for VariableSet resources.
type VariableSetService interface {
	List(ctx context.Context) (*v1alpha1.VariableSetList, error)
	Get(ctx context.Context, namespace, name string) (*v1alpha1.VariableSet, error)
}

type variableSetService struct {
	logger *slog.Logger
	client client.Client
}

func NewVariableSetService(logger *slog.Logger, client client.Client) VariableSetService {
	return &variableSetService{logger: logger, client: client}
}

func (s *variableSetService) List(ctx context.Context) (*v1alpha1.VariableSetList, error) {
	s.logger.Info("listing VariableSets")
	var list v1alpha1.VariableSetList
	if err := s.client.List(ctx, &list); err != nil {
		s.logger.Error("failed to list VariableSets", "error", err)
		return nil, err
	}
	s.logger.Info("VariableSets listed", "count", len(list.Items))
	return &list, nil
}

func (s *variableSetService) Get(ctx context.Context, namespace, name string) (*v1alpha1.VariableSet, error) {
	s.logger.Info("getting VariableSet", "namespace", namespace, "name", name)
	var obj v1alpha1.VariableSet
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &obj); err != nil {
		s.logger.Error("failed to get VariableSet", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}
	s.logger.Info("VariableSet retrieved", "namespace", namespace, "name", name)
	return &obj, nil
}

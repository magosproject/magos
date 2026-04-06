package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/magosproject/magos/api/internal/generated/clientset/versioned"
	"github.com/magosproject/magos/api/internal/generated/informers/externalversions"
	listerv1alpha1 "github.com/magosproject/magos/api/internal/generated/listers/types/v1alpha1"
	apiv1alpha1 "github.com/magosproject/magos/types/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// VariableSetService defines operations for VariableSet resources.
type VariableSetService interface {
	List(ctx context.Context) ([]*apiv1alpha1.VariableSet, error)
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.VariableSet, error)
}

type variableSetService struct {
	logger   *slog.Logger
	factory  externalversions.SharedInformerFactory
	informer cache.SharedIndexInformer
	lister   listerv1alpha1.VariableSetLister
}

func NewVariableSetService(logger *slog.Logger, client versioned.Interface) VariableSetService {
	factory := externalversions.NewSharedInformerFactory(client, 5*time.Minute)
	variableSetInformer := factory.Types().V1alpha1().VariableSets()

	svc := &variableSetService{
		logger:   logger,
		factory:  factory,
		informer: variableSetInformer.Informer(),
		lister:   variableSetInformer.Lister(),
	}

	svc.factory.Start(context.Background().Done())

	return svc
}

func (s *variableSetService) HasSynced() bool {
	return s.informer.HasSynced()
}

func (s *variableSetService) List(_ context.Context) ([]*apiv1alpha1.VariableSet, error) {
	variableSets, err := s.lister.List(labels.Everything())
	if err != nil {
		s.logger.Error("failed to list VariableSets", "error", err)
		return nil, err
	}
	s.logger.Info("VariableSets listed", "count", len(variableSets))
	return variableSets, nil
}

func (s *variableSetService) Get(_ context.Context, namespace, name string) (*apiv1alpha1.VariableSet, error) {
	variableSet, err := s.lister.VariableSets(namespace).Get(name)
	if err != nil {
		s.logger.Error("failed to get VariableSet", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}
	return variableSet, nil
}

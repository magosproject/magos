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

// RolloutService defines operations for Rollout resources.
type RolloutService interface {
	List(ctx context.Context) ([]*apiv1alpha1.Rollout, error)
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.Rollout, error)
}

type rolloutService struct {
	logger   *slog.Logger
	factory  externalversions.SharedInformerFactory
	informer cache.SharedIndexInformer
	lister   listerv1alpha1.RolloutLister
}

func NewRolloutService(logger *slog.Logger, client versioned.Interface) RolloutService {
	factory := externalversions.NewSharedInformerFactory(client, 30*time.Second)
	rolloutInformer := factory.Types().V1alpha1().Rollouts()

	svc := &rolloutService{
		logger:   logger,
		factory:  factory,
		informer: rolloutInformer.Informer(),
		lister:   rolloutInformer.Lister(),
	}

	svc.factory.Start(context.Background().Done())

	return svc
}

func (s *rolloutService) HasSynced() bool {
	return s.informer.HasSynced()
}

func (s *rolloutService) List(_ context.Context) ([]*apiv1alpha1.Rollout, error) {
	rollouts, err := s.lister.List(labels.Everything())
	if err != nil {
		s.logger.Error("failed to list Rollouts", "error", err)
		return nil, err
	}
	s.logger.Info("Rollouts listed", "count", len(rollouts))
	return rollouts, nil
}

func (s *rolloutService) Get(_ context.Context, namespace, name string) (*apiv1alpha1.Rollout, error) {
	rollout, err := s.lister.Rollouts(namespace).Get(name)
	if err != nil {
		s.logger.Error("failed to get Rollout", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}
	return rollout, nil
}

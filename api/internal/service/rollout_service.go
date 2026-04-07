package service

import (
	"context"
	"log/slog"

	"github.com/magosproject/magos/api/internal/generated/informers/externalversions"
	listerv1alpha1 "github.com/magosproject/magos/api/internal/generated/listers/magosproject/v1alpha1"
	apiv1alpha1 "github.com/magosproject/magos/types/magosproject/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type RolloutEvent struct {
	Type   watch.EventType      `json:"type"`
	Object *apiv1alpha1.Rollout `json:"object"`
}

// RolloutService defines operations for Rollout resources.
type RolloutService interface {
	Watch(ctx context.Context) <-chan RolloutEvent
	List(ctx context.Context) ([]*apiv1alpha1.Rollout, error)
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.Rollout, error)
}

type rolloutService struct {
	logger   *slog.Logger
	informer cache.SharedIndexInformer
	lister   listerv1alpha1.RolloutLister
	events   *Broadcaster[RolloutEvent]
}

func NewRolloutService(logger *slog.Logger, factory externalversions.SharedInformerFactory) RolloutService {
	rolloutInformer := factory.Magosproject().V1alpha1().Rollouts()

	svc := &rolloutService{
		logger:   logger,
		informer: rolloutInformer.Informer(),
		lister:   rolloutInformer.Lister(),
		events:   NewBroadcaster[RolloutEvent](),
	}

	rolloutInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc.events.Send(RolloutEvent{Type: watch.Added, Object: obj.(*apiv1alpha1.Rollout)})
		},
		UpdateFunc: func(_, obj interface{}) {
			svc.events.Send(RolloutEvent{Type: watch.Modified, Object: obj.(*apiv1alpha1.Rollout)})
		},
		DeleteFunc: func(obj interface{}) {
			rollout, ok := obj.(*apiv1alpha1.Rollout)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				rollout, ok = tombstone.Obj.(*apiv1alpha1.Rollout)
				if !ok {
					return
				}
			}
			svc.events.Send(RolloutEvent{Type: watch.Deleted, Object: rollout})
		},
	})

	return svc
}

func (s *rolloutService) HasSynced() bool {
	return s.informer.HasSynced()
}

func (s *rolloutService) Watch(ctx context.Context) <-chan RolloutEvent {
	return s.events.Subscribe(ctx)
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

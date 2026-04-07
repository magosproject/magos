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

type VariableSetEvent struct {
	Type   watch.EventType          `json:"type"`
	Object *apiv1alpha1.VariableSet `json:"object"`
}

// VariableSetService defines operations for VariableSet resources.
type VariableSetService interface {
	Watch(ctx context.Context) <-chan VariableSetEvent
	List(ctx context.Context) ([]*apiv1alpha1.VariableSet, error)
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.VariableSet, error)
}

type variableSetService struct {
	logger   *slog.Logger
	informer cache.SharedIndexInformer
	lister   listerv1alpha1.VariableSetLister
	events   *Broadcaster[VariableSetEvent]
}

func NewVariableSetService(logger *slog.Logger, factory externalversions.SharedInformerFactory) VariableSetService {
	variableSetInformer := factory.Magosproject().V1alpha1().VariableSets()

	svc := &variableSetService{
		logger:   logger,
		informer: variableSetInformer.Informer(),
		lister:   variableSetInformer.Lister(),
		events:   NewBroadcaster[VariableSetEvent](),
	}

	variableSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc.events.Send(VariableSetEvent{Type: watch.Added, Object: obj.(*apiv1alpha1.VariableSet)})
		},
		UpdateFunc: func(_, obj interface{}) {
			svc.events.Send(VariableSetEvent{Type: watch.Modified, Object: obj.(*apiv1alpha1.VariableSet)})
		},
		DeleteFunc: func(obj interface{}) {
			vs, ok := obj.(*apiv1alpha1.VariableSet)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				vs, ok = tombstone.Obj.(*apiv1alpha1.VariableSet)
				if !ok {
					return
				}
			}
			svc.events.Send(VariableSetEvent{Type: watch.Deleted, Object: vs})
		},
	})

	return svc
}

func (s *variableSetService) HasSynced() bool {
	return s.informer.HasSynced()
}

func (s *variableSetService) Watch(ctx context.Context) <-chan VariableSetEvent {
	return s.events.Subscribe(ctx)
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

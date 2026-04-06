package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/magosproject/magos/api/internal/generated/clientset/versioned"
	"github.com/magosproject/magos/api/internal/generated/informers/externalversions"
	listerv1alpha1 "github.com/magosproject/magos/api/internal/generated/listers/magosproject/v1alpha1"
	apiv1alpha1 "github.com/magosproject/magos/types/magosproject/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type WorkspaceEvent struct {
	Type   watch.EventType
	Object *apiv1alpha1.Workspace
}

// WorkspaceService defines operations for Workspace resources.
type WorkspaceService interface {
	HasSynced() bool
	Watch(ctx context.Context) <-chan WorkspaceEvent
	List(ctx context.Context) ([]*apiv1alpha1.Workspace, error)
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.Workspace, error)
}

type workspaceService struct {
	logger      *slog.Logger
	factory     externalversions.SharedInformerFactory
	informer    cache.SharedIndexInformer
	lister      listerv1alpha1.WorkspaceLister
	mu          sync.Mutex
	subscribers map[chan WorkspaceEvent]struct{}
}

func NewWorkspaceService(logger *slog.Logger, client versioned.Interface) WorkspaceService {
	factory := externalversions.NewSharedInformerFactory(client, 5*time.Minute)
	workspaceInformer := factory.Magosproject().V1alpha1().Workspaces()

	svc := &workspaceService{
		logger:      logger,
		lister:      workspaceInformer.Lister(),
		informer:    workspaceInformer.Informer(),
		factory:     factory,
		subscribers: make(map[chan WorkspaceEvent]struct{}),
	}

	workspaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc.broadcast(WorkspaceEvent{Type: watch.Added, Object: obj.(*apiv1alpha1.Workspace)})
		},
		UpdateFunc: func(_, obj interface{}) {
			svc.broadcast(WorkspaceEvent{Type: watch.Modified, Object: obj.(*apiv1alpha1.Workspace)})
		},
		DeleteFunc: func(obj interface{}) {
			workspace, ok := obj.(*apiv1alpha1.Workspace)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				workspace, ok = tombstone.Obj.(*apiv1alpha1.Workspace)
				if !ok {
					return
				}
			}
			svc.broadcast(WorkspaceEvent{Type: watch.Deleted, Object: workspace})
		},
	})

	svc.factory.Start(context.Background().Done())

	return svc
}

func (s *workspaceService) HasSynced() bool {
	return s.informer.HasSynced()
}

func (s *workspaceService) Watch(ctx context.Context) <-chan WorkspaceEvent {
	ch := make(chan WorkspaceEvent, 64)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		delete(s.subscribers, ch)
		close(ch)
		s.mu.Unlock()
	}()

	return ch
}

func (s *workspaceService) broadcast(event WorkspaceEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *workspaceService) List(_ context.Context) ([]*apiv1alpha1.Workspace, error) {
	workspaces, err := s.lister.List(labels.Everything())
	if err != nil {
		s.logger.Error("failed to list Workspaces", "error", err)
		return nil, err
	}
	s.logger.Info("Workspaces listed", "count", len(workspaces))
	return workspaces, nil
}

func (s *workspaceService) Get(_ context.Context, namespace, name string) (*apiv1alpha1.Workspace, error) {
	workspace, err := s.lister.Workspaces(namespace).Get(name)
	if err != nil {
		s.logger.Error("failed to get Workspace", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}
	return workspace, nil
}

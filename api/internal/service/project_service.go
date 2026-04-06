package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/magosproject/magos/api/internal/generated/clientset/versioned"
	"github.com/magosproject/magos/api/internal/generated/informers/externalversions"
	listerv1alpha1 "github.com/magosproject/magos/api/internal/generated/listers/types/v1alpha1"
	apiv1alpha1 "github.com/magosproject/magos/types/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type ProjectEvent struct {
	Type   watch.EventType
	Object *apiv1alpha1.Project
}

// ProjectService defines operations for Project resources.
type ProjectService interface {
	Watch(ctx context.Context) <-chan ProjectEvent
	List(ctx context.Context) ([]*apiv1alpha1.Project, error)
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.Project, error)
}

// projectService implements ProjectService.
type projectService struct {
	logger      *slog.Logger
	factory     externalversions.SharedInformerFactory
	informer    cache.SharedIndexInformer
	lister      listerv1alpha1.ProjectLister
	mu          sync.Mutex
	subscribers map[chan ProjectEvent]struct{}
}

// NewProjectService returns a new ProjectService.
func NewProjectService(logger *slog.Logger, client versioned.Interface) ProjectService {
	factory := externalversions.NewSharedInformerFactory(client, 5*time.Minute)
	projectInformer := factory.Types().V1alpha1().Projects()

	svc := &projectService{
		logger:      logger,
		lister:      projectInformer.Lister(),
		informer:    projectInformer.Informer(),
		factory:     factory,
		subscribers: make(map[chan ProjectEvent]struct{}),
	}

	projectInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc.broadcast(ProjectEvent{Type: watch.Added, Object: obj.(*apiv1alpha1.Project)})
		},
		UpdateFunc: func(_, obj interface{}) {
			svc.broadcast(ProjectEvent{Type: watch.Modified, Object: obj.(*apiv1alpha1.Project)})
		},
		DeleteFunc: func(obj interface{}) {
			project, ok := obj.(*apiv1alpha1.Project)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				project, ok = tombstone.Obj.(*apiv1alpha1.Project)
				if !ok {
					return
				}
			}
			svc.broadcast(ProjectEvent{Type: watch.Deleted, Object: project})
		},
	})

	svc.factory.Start(context.Background().Done())

	return svc
}

// HasSynced reports whether the informer cache has completed its initial sync.
func (s *projectService) HasSynced() bool {
	return s.informer.HasSynced()
}

// Watch returns a channel that receives events for Project resources.
func (s *projectService) Watch(ctx context.Context) <-chan ProjectEvent {
	ch := make(chan ProjectEvent, 64)
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

func (s *projectService) broadcast(event ProjectEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// List returns all Project resources across all namespaces.
func (s *projectService) List(_ context.Context) ([]*apiv1alpha1.Project, error) {
	projects, err := s.lister.List(labels.Everything())
	if err != nil {
		s.logger.Error("failed to list Projects", "error", err)
		return nil, err
	}
	s.logger.Info("Projects listed", "count", len(projects))
	return projects, nil
}

// Get returns a single Project resource by namespace and name.
func (s *projectService) Get(_ context.Context, namespace, name string) (*apiv1alpha1.Project, error) {
	project, err := s.lister.Projects(namespace).Get(name)
	if err != nil {
		s.logger.Error("failed to get Project", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}
	return project, nil
}

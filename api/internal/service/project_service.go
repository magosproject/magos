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

type ProjectEvent struct {
	Type   watch.EventType      `json:"type"`
	Object *apiv1alpha1.Project `json:"object"`
}

// ProjectService defines operations for Project resources.
type ProjectService interface {
	Watch(ctx context.Context) <-chan ProjectEvent
	List(ctx context.Context) ([]*apiv1alpha1.Project, error)
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.Project, error)
}

// projectService implements ProjectService.
type projectService struct {
	logger   *slog.Logger
	informer cache.SharedIndexInformer
	lister   listerv1alpha1.ProjectLister
	events   *Broadcaster[ProjectEvent]
}

// NewProjectService returns a new ProjectService.
func NewProjectService(logger *slog.Logger, factory externalversions.SharedInformerFactory) ProjectService {
	projectInformer := factory.Magosproject().V1alpha1().Projects()

	svc := &projectService{
		logger:   logger,
		lister:   projectInformer.Lister(),
		informer: projectInformer.Informer(),
		events:   NewBroadcaster[ProjectEvent](),
	}

	projectInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc.events.Send(ProjectEvent{Type: watch.Added, Object: obj.(*apiv1alpha1.Project)})
		},
		UpdateFunc: func(_, obj interface{}) {
			svc.events.Send(ProjectEvent{Type: watch.Modified, Object: obj.(*apiv1alpha1.Project)})
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
			svc.events.Send(ProjectEvent{Type: watch.Deleted, Object: project})
		},
	})

	return svc
}

// HasSynced reports whether the informer cache has completed its initial sync.
func (s *projectService) HasSynced() bool {
	return s.informer.HasSynced()
}

// Watch returns a channel that receives events for Project resources.
func (s *projectService) Watch(ctx context.Context) <-chan ProjectEvent {
	return s.events.Subscribe(ctx)
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

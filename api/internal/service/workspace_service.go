package service

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/magosproject/magos/api/internal/generated/clientset/versioned"
	"github.com/magosproject/magos/api/internal/generated/informers/externalversions"
	listerv1alpha1 "github.com/magosproject/magos/api/internal/generated/listers/magosproject/v1alpha1"
	"github.com/magosproject/magos/internal/logstore"
	apiv1alpha1 "github.com/magosproject/magos/types/magosproject/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type WorkspaceEvent struct {
	Type   watch.EventType        `json:"type"`
	Object *apiv1alpha1.Workspace `json:"object"`
}

// WorkspaceService defines operations for Workspace resources.
type WorkspaceService interface {
	HasSynced() bool
	Watch(ctx context.Context) <-chan WorkspaceEvent
	List(ctx context.Context) ([]*apiv1alpha1.Workspace, error)
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.Workspace, error)
	RequestReconcile(ctx context.Context, namespace, name string) (*apiv1alpha1.Workspace, error)
	ListRunLogs(ctx context.Context, namespace, name string, phase apiv1alpha1.RunPhase, limit int, cursor string) (*RunLogListResponse, error)
	GetRunLog(ctx context.Context, namespace, name, runID string, phase apiv1alpha1.RunPhase) (io.ReadCloser, error)
}

type RunLogListResponse struct {
	Items      []apiv1alpha1.RunLogSummary `json:"items"`
	NextCursor string                      `json:"nextCursor,omitempty"`
}

type workspaceService struct {
	logger   *slog.Logger
	client   versioned.Interface
	informer cache.SharedIndexInformer
	lister   listerv1alpha1.WorkspaceLister
	events   *Broadcaster[WorkspaceEvent]
	logStore logstore.Store
}

func NewWorkspaceService(logger *slog.Logger, factory externalversions.SharedInformerFactory, client versioned.Interface, logs logstore.Store) WorkspaceService {
	workspaceInformer := factory.Magosproject().V1alpha1().Workspaces()

	svc := &workspaceService{
		logger:   logger,
		client:   client,
		lister:   workspaceInformer.Lister(),
		informer: workspaceInformer.Informer(),
		events:   NewBroadcaster[WorkspaceEvent](),
		logStore: logs,
	}

	workspaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc.events.Send(WorkspaceEvent{Type: watch.Added, Object: obj.(*apiv1alpha1.Workspace)})
		},
		UpdateFunc: func(_, obj interface{}) {
			svc.events.Send(WorkspaceEvent{Type: watch.Modified, Object: obj.(*apiv1alpha1.Workspace)})
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
			svc.events.Send(WorkspaceEvent{Type: watch.Deleted, Object: workspace})
		},
	})

	return svc
}

func (s *workspaceService) HasSynced() bool {
	return s.informer.HasSynced()
}

func (s *workspaceService) Watch(ctx context.Context) <-chan WorkspaceEvent {
	return s.events.Subscribe(ctx)
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

func (s *workspaceService) RequestReconcile(ctx context.Context, namespace, name string) (*apiv1alpha1.Workspace, error) {
	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				apiv1alpha1.WorkspaceReconcileRequestAnnotation: time.Now().UTC().Format(time.RFC3339Nano),
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	workspace, err := s.client.MagosprojectV1alpha1().Workspaces(namespace).Patch(
		ctx,
		name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		s.logger.Error("failed to request Workspace reconcile", "error", err, "namespace", namespace, "name", name)
		return nil, err
	}

	return workspace, nil
}

func (s *workspaceService) ListRunLogs(ctx context.Context, namespace, name string, phase apiv1alpha1.RunPhase, limit int, cursor string) (*RunLogListResponse, error) {
	workspace, err := s.Get(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	if s.logStore == nil {
		return &RunLogListResponse{}, nil
	}
	result, err := s.logStore.ListRunSummaries(ctx, logstore.ListRunSummariesInput{
		Namespace: workspace.Namespace,
		Workspace: workspace.Name,
		Phase:     phase,
		Limit:     limit,
		Cursor:    cursor,
	})
	if err != nil {
		return nil, err
	}
	return &RunLogListResponse{
		Items:      result.Items,
		NextCursor: result.NextCursor,
	}, nil
}

func (s *workspaceService) GetRunLog(ctx context.Context, namespace, name, runID string, phase apiv1alpha1.RunPhase) (io.ReadCloser, error) {
	if s.logStore == nil {
		return nil, fmt.Errorf("run log storage is not configured")
	}

	workspace, err := s.Get(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	item, err := s.logStore.FindRunSummary(ctx, workspace.Namespace, workspace.Name, phase, runID)
	if err != nil {
		return nil, err
	}
	body, _, err := s.logStore.Get(ctx, item.LogKey)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(body)
	if err != nil {
		_ = body.Close()
		return nil, fmt.Errorf("open gzip run log: %w", err)
	}
	return &gzipReadCloser{Reader: gz, body: body}, nil
}

type gzipReadCloser struct {
	Reader *gzip.Reader
	body   io.Closer
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.Reader.Read(p)
}

func (g *gzipReadCloser) Close() error {
	readerErr := g.Reader.Close()
	bodyErr := g.body.Close()
	if readerErr != nil {
		return readerErr
	}
	return bodyErr
}

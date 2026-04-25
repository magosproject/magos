package service

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"time"

	"github.com/magosproject/magos/api/internal/generated/clientset/versioned"
	"github.com/magosproject/magos/api/internal/generated/informers/externalversions"
	listerv1alpha1 "github.com/magosproject/magos/api/internal/generated/listers/magosproject/v1alpha1"
	"github.com/magosproject/magos/internal/logstore"
	apiv1alpha1 "github.com/magosproject/magos/types/magosproject/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
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
	ListReconcileRuns(ctx context.Context, namespace, name string, limit int, cursor string) (*ReconcileRunListResponse, error)
	GetRunPhaseLog(ctx context.Context, namespace, name, runID string, phase apiv1alpha1.RunPhase) (io.ReadCloser, error)
	StreamCurrentRunLogs(ctx context.Context, namespace, name string, phase apiv1alpha1.RunPhase) <-chan RunLogStreamEvent
}

type ReconcileRunListResponse struct {
	Items      []apiv1alpha1.ReconcileRun `json:"items"`
	NextCursor string                     `json:"nextCursor,omitempty"`
}

type RunLogStreamEvent struct {
	Type    string               `json:"type"`
	RunID   string               `json:"runID,omitempty"`
	Phase   apiv1alpha1.RunPhase `json:"phase,omitempty"`
	PodName string               `json:"podName,omitempty"`
	Line    string               `json:"line,omitempty"`
	Message string               `json:"message,omitempty"`
}

type workspaceService struct {
	logger   *slog.Logger
	client   versioned.Interface
	kube     kubernetes.Interface
	informer cache.SharedIndexInformer
	lister   listerv1alpha1.WorkspaceLister
	events   *Broadcaster[WorkspaceEvent]
	logStore logstore.Store
}

func NewWorkspaceService(logger *slog.Logger, factory externalversions.SharedInformerFactory, client versioned.Interface, kube kubernetes.Interface, logs logstore.Store) WorkspaceService {
	workspaceInformer := factory.Magosproject().V1alpha1().Workspaces()

	svc := &workspaceService{
		logger:   logger,
		client:   client,
		kube:     kube,
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

func (s *workspaceService) ListReconcileRuns(ctx context.Context, namespace, name string, limit int, cursor string) (*ReconcileRunListResponse, error) {
	workspace, err := s.Get(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	if s.logStore == nil {
		return &ReconcileRunListResponse{}, nil
	}
	runs, nextCursor, err := s.logStore.ListReconcileRuns(ctx, workspace.Namespace, workspace.Name, limit, cursor)
	if err != nil {
		return nil, err
	}
	return &ReconcileRunListResponse{
		Items:      runs,
		NextCursor: nextCursor,
	}, nil
}

func (s *workspaceService) GetRunPhaseLog(ctx context.Context, namespace, name, runID string, phase apiv1alpha1.RunPhase) (io.ReadCloser, error) {
	if s.logStore == nil {
		return nil, fmt.Errorf("run log storage is not configured")
	}

	workspace, err := s.Get(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	key := logstore.RunLogKey(workspace.Namespace, workspace.Name, runID, phase)
	body, err := s.logStore.GetRunPhaseLog(ctx, key)
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

func (s *workspaceService) StreamCurrentRunLogs(ctx context.Context, namespace, name string, phase apiv1alpha1.RunPhase) <-chan RunLogStreamEvent {
	ch := make(chan RunLogStreamEvent)

	go func() {
		defer close(ch)

		if s.kube == nil {
			sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "error", Phase: phase, Message: "kubernetes client is not configured"})
			return
		}

		workspace, err := s.Get(ctx, namespace, name)
		if err != nil {
			sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "error", Phase: phase, Message: "workspace not found"})
			return
		}

		runID := workspace.Status.CurrentRunID
		if runID == "" {
			sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "status", Phase: phase, Message: "no active run"})
			return
		}

		sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "status", RunID: runID, Phase: phase, Message: "waiting for log stream"})

		pod, err := s.waitForRunPod(ctx, namespace, name, runID, phase)
		if err != nil {
			sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "error", RunID: runID, Phase: phase, Message: err.Error()})
			return
		}

		sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "status", RunID: runID, Phase: phase, PodName: pod.Name, Message: "streaming"})

		req := s.kube.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: "job",
			Follow:    true,
		})
		stream, err := req.Stream(ctx)
		if err != nil {
			sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "error", RunID: runID, Phase: phase, PodName: pod.Name, Message: fmt.Sprintf("open pod logs: %v", err)})
			return
		}
		defer func() { _ = stream.Close() }()

		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			if !sendRunLogEvent(ctx, ch, RunLogStreamEvent{
				Type:    "line",
				RunID:   runID,
				Phase:   phase,
				PodName: pod.Name,
				Line:    scanner.Text(),
			}) {
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "error", RunID: runID, Phase: phase, PodName: pod.Name, Message: fmt.Sprintf("read pod logs: %v", err)})
			return
		}

		sendRunLogEvent(ctx, ch, RunLogStreamEvent{Type: "eof", RunID: runID, Phase: phase, PodName: pod.Name, Message: "log stream completed"})
	}()

	return ch
}

func sendRunLogEvent(ctx context.Context, ch chan<- RunLogStreamEvent, event RunLogStreamEvent) bool {
	select {
	case ch <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *workspaceService) waitForRunPod(ctx context.Context, namespace, workspaceName, runID string, phase apiv1alpha1.RunPhase) (*corev1.Pod, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		workspace, err := s.Get(ctx, namespace, workspaceName)
		if err != nil {
			return nil, err
		}
		if workspace.Status.CurrentRunID != runID {
			return nil, fmt.Errorf("run changed while waiting for log stream")
		}

		pod, err := s.findRunPod(ctx, namespace, workspaceName, runID, phase)
		if err == nil {
			return pod, nil
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (s *workspaceService) findRunPod(ctx context.Context, namespace, workspaceName, runID string, phase apiv1alpha1.RunPhase) (*corev1.Pod, error) {
	selector := labels.Set{
		"magosproject.io/workspace": workspaceName,
		"magosproject.io/job-type":  string(phase),
		"magosproject.io/run-id":    runID,
	}.String()

	pods, err := s.kube.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pod found for current %s run yet", phase)
	}

	sort.SliceStable(pods.Items, func(i, j int) bool {
		return pods.Items[i].CreationTimestamp.Time.After(pods.Items[j].CreationTimestamp.Time)
	})

	for i := range pods.Items {
		pod := &pods.Items[i]
		switch pod.Status.Phase {
		case corev1.PodPending, corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed:
			return pod, nil
		}
	}

	return &pods.Items[0], nil
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

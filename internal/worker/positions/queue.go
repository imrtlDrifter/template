package positions

import (
	"bytes"
	"container/heap"
	"context"
	"sync"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type task struct {
	link  trace.SpanContext
	trip  models.StaleTrip
	index int
}

type taskHeap []*task

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	a, b := h[i].trip, h[j].trip
	if a.Since.Equal(b.Since) {
		return bytes.Compare(a.ID[:], b.ID[:]) < 0
	}
	return a.Since.Before(b.Since)
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

func (h *taskHeap) Push(value any) {
	t := value.(*task)
	t.index = len(*h)
	*h = append(*h, t)
}

func (h *taskHeap) Pop() any {
	items := *h
	last := len(items) - 1
	t := items[last]
	t.index = -1
	items[last] = nil
	*h = items[:last]
	return t
}

type queue struct {
	ready   *sync.Cond
	byID    map[uuid.UUID]*task
	metrics positionMetrics
	tasks   taskHeap
	active  int
	closed  bool
}

func newQueue(capacity int, metrics positionMetrics) *queue {
	return &queue{
		ready:   sync.NewCond(&sync.Mutex{}),
		tasks:   make(taskHeap, 0, capacity),
		byID:    make(map[uuid.UUID]*task),
		metrics: metrics,
	}
}

func (q *queue) enqueue(ctx context.Context, trip models.StaleTrip) (result string, evicted bool) {
	q.ready.L.Lock()
	defer q.ready.L.Unlock()

	if _, exists := q.byID[trip.ID]; exists {
		return duplicate, false
	}

	if len(q.tasks) == cap(q.tasks) {
		// In a min-heap, the newest task is among the leaves.
		newest := len(q.tasks) / 2
		for idx := newest + 1; idx < len(q.tasks); idx++ {
			if q.tasks.Less(newest, idx) {
				newest = idx
			}
		}
		if !trip.Since.Before(q.tasks[newest].trip.Since) {
			return dropped, false
		}
		delete(q.byID, heap.Remove(&q.tasks, newest).(*task).trip.ID)
		evicted = true
	}

	t := &task{trip: trip, link: trace.SpanContextFromContext(ctx)}
	heap.Push(&q.tasks, t)
	q.byID[trip.ID] = t
	q.metrics.QueueSize(ctx, len(q.tasks))
	q.ready.Signal()
	return queued, evicted
}

func (q *queue) take(ctx context.Context) *task {
	q.ready.L.Lock()
	defer q.ready.L.Unlock()

	for len(q.tasks) == 0 && !q.closed {
		q.ready.Wait()
	}
	if len(q.tasks) == 0 {
		return nil
	}

	t := heap.Pop(&q.tasks).(*task)
	q.active++
	q.metrics.QueueSize(ctx, len(q.tasks))
	q.metrics.WorkersActive(ctx, q.active)
	return t
}

func (q *queue) finish(ctx context.Context, t *task) {
	q.ready.L.Lock()
	defer q.ready.L.Unlock()

	delete(q.byID, t.trip.ID)
	q.active--
	q.metrics.WorkersActive(ctx, q.active)
}

func (q *queue) cancel(ctx context.Context, tripID uuid.UUID) bool {
	q.ready.L.Lock()
	defer q.ready.L.Unlock()

	t, exists := q.byID[tripID]
	if !exists || t.index < 0 {
		return false
	}
	heap.Remove(&q.tasks, t.index)
	delete(q.byID, tripID)
	q.metrics.QueueSize(ctx, len(q.tasks))
	return true
}

func (q *queue) close() {
	q.ready.L.Lock()
	defer q.ready.L.Unlock()

	q.closed = true
	q.ready.Broadcast()
}

package positions

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/imrtlDrifter/template/internal/config"
	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestPriorityQueue(t *testing.T) {
	ctx := t.Context()
	now := time.Now()
	trips := makeTrips(now, 5)
	q := newQueue(3, &testMetrics{})
	for _, i := range []int{2, 4, 3} {
		if result, evicted := q.enqueue(ctx, trips[i]); result != queued || evicted {
			t.Fatalf("enqueue %d = %s, evicted = %v", i, result, evicted)
		}
	}
	if result, evicted := q.enqueue(ctx, trips[0]); result != queued || !evicted {
		t.Fatalf("older trip = %s, evicted = %v", result, evicted)
	}
	if result, evicted := q.enqueue(ctx, trips[4]); result != dropped || evicted {
		t.Fatalf("newest trip = %s, evicted = %v", result, evicted)
	}
	if !q.cancel(ctx, trips[2].ID) {
		t.Fatal("queued trip was not canceled")
	}
	q.close()
	for _, i := range []int{0, 3} {
		task := q.take(ctx)
		if task == nil || task.trip.ID != trips[i].ID {
			t.Fatalf("take = %v, want trip %d", task, i)
		}
		q.finish(ctx, task)
	}
	if task := q.take(ctx); task != nil {
		t.Fatalf("unexpected task %v", task)
	}
}

func TestScanBatches(t *testing.T) {
	for _, test := range []struct {
		name     string
		capacity int
		queued   int
		rows     int
		calls    int
		drops    int
	}{
		{name: "three pages", capacity: 250, queued: 250, rows: 250, calls: 3},
		{name: "first drop stops scan", capacity: 50, queued: 50, rows: 100, calls: 1, drops: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now()
			metrics := &testMetrics{}
			ctrl := gomock.NewController(t)
			repo := NewMocktripRepository(ctrl)
			trips := makeTrips(now, 250)
			for i := range trips {
				trips[i].Since = now
			}
			calls := make([]any, 0, test.calls)
			var cursor *models.StaleTrip
			for page := range test.calls {
				start := page * 100
				end := min(start+100, len(trips))
				calls = append(calls, repo.EXPECT().ListStaleTrips(gomock.Any(), now, time.Minute, cursor, 100).Return(trips[start:end], nil))
				cursor = &trips[end-1]
			}
			gomock.InOrder(calls...)
			worker := newTestWorker(ctrl, repo, nil, metrics, 1, test.capacity)
			worker.scanTick(t.Context(), now)
			if !reflect.DeepEqual(metrics.rows, []int{test.rows}) || metrics.checks[queued] != test.queued || metrics.checks[dropped] != test.drops {
				t.Fatalf("rows = %v, checks = %v", metrics.rows, metrics.checks)
			}
		})
	}
}

func TestWorkerDeduplicatesCancelsAndDrains(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		trips := makeTrips(now, 5)
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		gomock.InOrder(
			repo.EXPECT().ListStaleTrips(gomock.Any(), now, time.Minute, nil, 100).Return(trips[:4], nil),
			repo.EXPECT().ListStaleTrips(gomock.Any(), now.Add(time.Minute), time.Minute, nil, 100).Return(trips[:4], nil),
		)
		repo.EXPECT().StillStale(gomock.Any(), gomock.Any()).Return(true, nil).Times(3)
		metrics := &testMetrics{}
		started := make(chan uuid.UUID, len(trips))
		release := make(chan struct{})
		push := NewMockpushGateway(ctrl)
		push.EXPECT().RequestPosition(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, trip models.StaleTrip) error {
			started <- trip.ID
			select {
			case <-release:
				if trip.ID == trips[0].ID {
					return errors.New("push unavailable")
				}
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}).Times(3)

		worker := newTestWorker(ctrl, repo, push, metrics, 2, 4)
		ticks, stop, done := startWorker(t, worker, context.WithoutCancel(t.Context()))
		ticks <- now
		synctest.Wait()
		first, second := <-started, <-started
		if !slices.Contains([]uuid.UUID{first, second}, trips[0].ID) || !slices.Contains([]uuid.UUID{first, second}, trips[1].ID) || len(started) != 0 || metrics.active != 2 {
			t.Fatalf("expected two workers to take the two oldest trips, active = %d", metrics.active)
		}

		ticks <- now.Add(time.Minute)
		synctest.Wait()
		if metrics.checks[duplicate] != 4 {
			t.Fatalf("checks = %v", metrics.checks)
		}
		worker.Cancel(t.Context(), trips[2].ID)
		stop()
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("worker exited before draining")
		default:
		}

		close(release)
		synctest.Wait()
		<-done
		if len(started) != 1 || <-started != trips[3].ID {
			t.Fatal("expected only the remaining queued trip to be sent")
		}
		if metrics.requests[failed] != 1 || metrics.requests[success] != 2 || metrics.requests[dropped] != 1 || metrics.active != 0 {
			t.Fatalf("requests = %v, active = %d", metrics.requests, metrics.active)
		}
	})
}

func TestWorkerStopsOnDrainDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		trips := makeTrips(now, 2)
		repo.EXPECT().ListStaleTrips(gomock.Any(), now, time.Minute, nil, 100).Return(trips, nil)
		repo.EXPECT().StillStale(gomock.Any(), trips[0]).Return(true, nil)
		metrics := &testMetrics{}
		workCtx, cancel := context.WithCancel(t.Context())
		defer cancel()
		push := NewMockpushGateway(ctrl)
		push.EXPECT().RequestPosition(gomock.Any(), trips[0]).DoAndReturn(func(ctx context.Context, _ models.StaleTrip) error {
			<-ctx.Done()
			return ctx.Err()
		})
		worker := newTestWorker(ctrl, repo, push, metrics, 1, 2)
		ticks, stop, done := startWorker(t, worker, workCtx)
		ticks <- now
		synctest.Wait()
		stop()
		cancel()
		synctest.Wait()
		<-done
		if metrics.requests[failed] != 1 || metrics.requests[dropped] != 1 || metrics.active != 0 {
			t.Fatalf("requests = %v, active = %d", metrics.requests, metrics.active)
		}
	})
}

type testTicker struct{ ticks chan time.Time }

func (t testTicker) C() <-chan time.Time { return t.ticks }

func (testTicker) Stop() {}

type testMetrics struct {
	checks   map[string]int
	requests map[string]int
	rows     []int
	mu       sync.Mutex
	active   int
}

func (m *testMetrics) StaleCheck(_ context.Context, result string) { m.checks[result]++ }

func (m *testMetrics) RowsRead(_ context.Context, count int) { m.rows = append(m.rows, count) }

func (*testMetrics) QueueSize(context.Context, int) {}

func (m *testMetrics) WorkersActive(_ context.Context, count int) { m.active = count }

func (m *testMetrics) PositionRequest(_ context.Context, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[result]++
}

func (*testMetrics) PushDuration(context.Context, string, time.Duration) {}

func makeTrips(now time.Time, count int) []models.StaleTrip {
	trips := make([]models.StaleTrip, count)
	for i := range trips {
		trips[i] = models.StaleTrip{ID: uuid.New(), Since: now.Add(time.Duration(i) * time.Second)}
	}
	return trips
}

func newTestWorker(ctrl *gomock.Controller, repo tripRepository, push pushGateway, metrics *testMetrics, workers, capacity int) *Worker {
	metrics.checks = make(map[string]int)
	metrics.requests = make(map[string]int)
	log := NewMockworkerLogger(ctrl)
	log.EXPECT().WarnContext(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	return NewWorker(repo, push, metrics, log, noop.NewTracerProvider().Tracer("test"), config.Positions{
		Workers: workers, QueueSize: capacity, BatchSize: 100,
		CheckInterval: time.Minute, StaleAfter: time.Minute, PushTimeout: time.Second,
	})
}

func startWorker(t *testing.T, worker *Worker, workCtx context.Context) (chan<- time.Time, context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ticks := make(chan time.Time)
	worker.ticker = func(time.Duration) ticker { return testTicker{ticks: ticks} }
	ctx, stop := context.WithCancel(t.Context())
	t.Cleanup(stop)
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx, workCtx)
	}()
	return ticks, stop, done
}

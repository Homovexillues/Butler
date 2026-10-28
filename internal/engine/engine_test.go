package engine

import (
	"butler/internal/action"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNextSelectsEarliestAndSkipsRunning(t *testing.T) {
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	last := now.Add(-time.Hour)
	nodes := map[int64]*model.Node{
		9: {ID: 9},
		1: {ID: 1, Schedule: schedule.Once{At: now.Add(time.Hour)}},
		2: {ID: 2, Schedule: schedule.Once{At: now.Add(-time.Minute)}, LastFired: last},
		3: {ID: 3, Schedule: schedule.Once{At: now.Add(-2 * time.Hour)}},
	}
	running := map[int64]bool{}
	node, at, ok := Next(nodes, running, now)
	if !ok || node.ID != 2 || !at.Equal(now.Add(-time.Minute)) {
		t.Fatalf("next = %+v %v %v", node, at, ok)
	}
	if !nodes[2].LastFired.Equal(last) || len(running) != 0 {
		t.Fatal("Next modified inputs")
	}
	running[2] = true
	node, at, ok = Next(nodes, running, now)
	if !ok || node.ID != 1 || !at.Equal(now.Add(time.Hour)) {
		t.Fatalf("selected running task: %+v %v %v", node, at, ok)
	}
	if _, _, ok := Next(nil, nil, now); ok {
		t.Fatal("empty list has a next task")
	}
}

func TestNextUsesAttemptWhenNeverFired(t *testing.T) {
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	attempt := now.Add(-time.Hour)
	nodes := map[int64]*model.Node{
		1: {ID: 1, Schedule: schedule.Once{At: now.Add(-time.Minute)}, LastAttempt: attempt},
	}
	node, at, ok := Next(nodes, nil, now)
	if !ok || node.ID != 1 || !at.Equal(now.Add(-time.Minute)) {
		t.Fatalf("next = %+v %v %v", node, at, ok)
	}
}

func TestMergeNodesKeepsExecutionState(t *testing.T) {
	anchor := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	older := anchor.Add(-3 * time.Hour)
	newer := anchor.Add(-time.Hour)

	tests := []struct {
		name        string
		oldNodes    map[int64]*model.Node
		newNodes    []*model.Node
		wantAttempt time.Time
		wantFired   time.Time
	}{
		{
			name:        "reload keeps in-memory state",
			oldNodes:    map[int64]*model.Node{1: {ID: 1, LastAttempt: newer, LastFired: older}},
			newNodes:    []*model.Node{{ID: 1}},
			wantAttempt: newer,
			wantFired:   older,
		},
		{
			name:        "newer snapshot state wins",
			oldNodes:    map[int64]*model.Node{1: {ID: 1, LastAttempt: older}},
			newNodes:    []*model.Node{{ID: 1, LastAttempt: newer}},
			wantAttempt: newer,
		},
		{
			name:        "stale snapshot cannot rewind",
			oldNodes:    map[int64]*model.Node{1: {ID: 1, LastAttempt: newer}},
			newNodes:    []*model.Node{{ID: 1, LastAttempt: older}},
			wantAttempt: newer,
		},
		{
			name:        "legacy last fired becomes attempt",
			newNodes:    []*model.Node{{ID: 1, LastFired: older}},
			wantAttempt: older,
			wantFired:   older,
		},
		{
			name:        "never executed uses anchor",
			newNodes:    []*model.Node{{ID: 1}},
			wantAttempt: anchor,
		},
		{
			name:        "zero time is unset",
			newNodes:    []*model.Node{{ID: 1, LastAttempt: time.Time{}, LastFired: time.Time{}}},
			wantAttempt: anchor,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeNodes(tt.oldNodes, tt.newNodes, anchor)[1]
			if got == nil {
				t.Fatal("node missing")
			}
			if !got.LastAttempt.Equal(tt.wantAttempt) {
				t.Fatalf("attempt = %v, want %v", got.LastAttempt, tt.wantAttempt)
			}
			if !got.LastFired.Equal(tt.wantFired) {
				t.Fatalf("fired = %v, want %v", got.LastFired, tt.wantFired)
			}
		})
	}
}

type testAction struct {
	entered chan struct{}
	release <-chan struct{}
}

func (a testAction) Execute(ctx context.Context) (action.Result, error) {
	close(a.entered)
	select {
	case <-a.release:
		return action.Result{Output: "command output"}, nil
	case <-ctx.Done():
		return action.Result{}, ctx.Err()
	}
}

type failingAction struct{}

func (failingAction) Execute(context.Context) (action.Result, error) {
	return action.Result{Output: "boom"}, errors.New("boom")
}

type blockingFailAction struct {
	entered chan struct{}
	release <-chan struct{}
}

func (a blockingFailAction) Execute(ctx context.Context) (action.Result, error) {
	close(a.entered)
	select {
	case <-a.release:
		return action.Result{Output: "boom"}, errors.New("boom")
	case <-ctx.Done():
		return action.Result{}, ctx.Err()
	}
}

type savedState struct {
	id      int64
	attempt *time.Time
	fired   *time.Time
}

func startTestEngine(t *testing.T) (context.Context, context.CancelFunc, chan []*model.Node, <-chan notify.Request, <-chan savedState) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	nodes := make(chan []*model.Node, 4)
	requests := make(chan notify.Request, 4)
	saved := make(chan savedState, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, nodes, requests, func(id int64, attempt, fired *time.Time) error {
			saved <- savedState{id, attempt, fired}
			return nil
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("engine did not stop")
		}
	})
	return ctx, cancel, nodes, requests, saved
}

func TestRunLoadsReminderAndPersistsCompletion(t *testing.T) {
	ctx, _, updates, requests, saved := startTestEngine(t)
	at := time.Now().Add(100 * time.Millisecond)
	node := &model.Node{ID: 7, Title: "提醒", Body: "正文", Channels: []notify.Channel{notify.ChannelSystem}, Schedule: schedule.Once{At: at}}
	// Engine begins empty and waits for this initial snapshot.
	updates <- []*model.Node{node}
	select {
	case request := <-requests:
		if request.Message.Title != node.Title || request.Message.Body != node.Body || len(request.Channels) != 1 || request.Channels[0] != notify.ChannelSystem {
			t.Fatalf("request = %+v", request)
		}
	case <-ctx.Done():
		t.Fatal("no reminder")
	}
	select {
	case state := <-saved:
		if state.id != 7 || state.attempt == nil || state.attempt.Before(at) || state.fired == nil || state.fired.Before(at) {
			t.Fatalf("saved = %+v", state)
		}
	case <-ctx.Done():
		t.Fatal("state not saved")
	}
	// Snapshot copies are owned by the engine, not written through caller pointers.
	if !node.LastAttempt.IsZero() || !node.LastFired.IsZero() {
		t.Fatal("source node was mutated")
	}
}

func TestRunFailureRecordsAttemptWithoutRetryingImmediately(t *testing.T) {
	ctx, _, updates, requests, saved := startTestEngine(t)
	everyHour, err := schedule.NewIntervalSchedule("1h")
	if err != nil {
		t.Fatal(err)
	}
	// An old attempt makes the task due immediately; after the failure the next
	// run must be one interval later, not another immediate retry.
	past := time.Now().Add(-2 * time.Hour)
	node := &model.Node{
		ID: 9, Title: "失败", Channels: []notify.Channel{notify.ChannelSystem},
		Schedule: everyHour, Action: failingAction{}, LastAttempt: past,
	}
	updates <- []*model.Node{node}
	select {
	case <-requests:
	case <-ctx.Done():
		t.Fatal("no error notification")
	}
	select {
	case state := <-saved:
		if state.id != 9 || state.attempt == nil || state.fired != nil {
			t.Fatalf("saved = %+v", state)
		}
	case <-ctx.Done():
		t.Fatal("state not saved")
	}
	select {
	case request := <-requests:
		t.Fatalf("immediate retry: %+v", request)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRunFailureAfterReloadUpdatesCurrentSnapshot(t *testing.T) {
	ctx, _, updates, requests, saved := startTestEngine(t)
	everyHour, err := schedule.NewIntervalSchedule("1h")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	entered, release := make(chan struct{}), make(chan struct{})
	node := &model.Node{
		ID: 10, Title: "reload", Channels: []notify.Channel{notify.ChannelSystem},
		Schedule: everyHour, Action: blockingFailAction{entered, release}, LastAttempt: past,
	}
	updates <- []*model.Node{node}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("action not started")
	}
	// A reload while the action runs replaces the node pointer. The failure
	// result must update that new snapshot, or the old attempt makes it due again.
	replacement := *node
	replacement.Title = "reloaded"
	replacement.LastAttempt = time.Time{}
	updates <- []*model.Node{&replacement}
	close(release)
	select {
	case <-requests:
	case <-ctx.Done():
		t.Fatal("no error notification")
	}
	select {
	case state := <-saved:
		if state.id != 10 || state.attempt == nil || !state.attempt.After(past) || state.fired != nil {
			t.Fatalf("saved = %+v", state)
		}
	case <-ctx.Done():
		t.Fatal("state not saved")
	}
	select {
	case request := <-requests:
		t.Fatalf("immediate retry after reload: %+v", request)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRunCommandReloadWhileInFlight(t *testing.T) {
	ctx, _, updates, requests, saved := startTestEngine(t)
	entered, release := make(chan struct{}), make(chan struct{})
	node := &model.Node{ID: 8, Title: "命令", Channels: []notify.Channel{notify.ChannelMQTT}, Schedule: schedule.Once{At: time.Now().Add(100 * time.Millisecond)}, Action: testAction{entered, release}}
	updates <- []*model.Node{node}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("action not started")
	}
	replacement := *node
	replacement.Body = "new configuration"
	updates <- []*model.Node{&replacement}
	close(release)
	select {
	case request := <-requests:
		if !strings.Contains(request.Message.Body, "command output") {
			t.Fatalf("output missing: %+v", request)
		}
	case <-ctx.Done():
		t.Fatal("no command result")
	}
	select {
	case state := <-saved:
		if state.id != 8 || state.attempt == nil || state.fired == nil {
			t.Fatalf("saved = %+v", state)
		}
	case <-ctx.Done():
		t.Fatal("state not saved")
	}
	// Both snapshots describe the same once task, so no duplicate notification.
	select {
	case request := <-requests:
		t.Fatalf("duplicate: %+v", request)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRunCancellationWithClosedUpdates(t *testing.T) {
	_, cancel, updates, _, _ := startTestEngine(t)
	close(updates)
	cancel()
}

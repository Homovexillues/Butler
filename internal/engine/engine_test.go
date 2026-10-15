package engine

import (
	"butler/internal/action"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
	"context"
	"strings"
	"testing"
	"time"
)

func TestNextSelectsEarliestAndSkipsRunning(t *testing.T) {
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	last := now.Add(-time.Hour)
	nodes := []*model.Node{nil, {ID: 9},
		{ID: 1, Schedule: schedule.Once{At: now.Add(time.Hour)}},
		{ID: 2, Schedule: schedule.Once{At: now.Add(-time.Minute)}, LastFired: &last},
		{ID: 3, Schedule: schedule.Once{At: now.Add(-2 * time.Hour)}},
	}
	running := map[int64]bool{}
	index, at, ok := Next(nodes, running, now)
	if !ok || index != 3 || !at.Equal(now.Add(-time.Minute)) {
		t.Fatalf("next = %d %v %v", index, at, ok)
	}
	if *nodes[3].LastFired != last || len(running) != 0 {
		t.Fatal("Next modified inputs")
	}
	running[2] = true
	index, _, ok = Next(nodes, running, now)
	if !ok || index != 2 {
		t.Fatalf("selected running task: %d %v", index, ok)
	}
	if _, _, ok := Next(nil, nil, now); ok {
		t.Fatal("empty list has a next task")
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

type savedState struct {
	id    int64
	fired time.Time
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
		Run(ctx, nodes, requests, func(id int64, at time.Time) error { saved <- savedState{id, at}; return nil })
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
		if state.id != 7 || state.fired.Before(at) {
			t.Fatalf("saved = %+v", state)
		}
	case <-ctx.Done():
		t.Fatal("state not saved")
	}
	// Snapshot copies are owned by the engine, not written through caller pointers.
	if node.LastFired != nil {
		t.Fatal("source node was mutated")
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
		if state.id != 8 {
			t.Fatal(state)
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

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
)

func TestTaskRoundTrip(t *testing.T) {
	s, err := NewStore(filepath.Join(t.TempDir(), "butler.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	want := model.Task{
		Title: "reminder", Body: "hello", Enabled: true,
		ScheduleType:       schedule.ScheduleSolar,
		ScheduleExpression: "03-05 09:00:00",
		ScheduleOffset:     []string{"T-3d", "T-0d"},
		ActionType:         "",
		ActionExpression:   json.RawMessage(`{"channels":["mqtt"]}`),
		Channels:           []notify.Channel{notify.ChannelMQTT},
	}
	want.ID, err = s.Add(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		got, err := s.Get(ctx, want.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
	check()
	want.ScheduleOffset = []string{}
	want.ActionExpression = json.RawMessage(`{"channels":["email"]}`)
	want.Channels = []notify.Channel{notify.ChannelEmail}
	want.Enabled = false
	if err := s.Update(ctx, want); err != nil {
		t.Fatal(err)
	}
	check()
}

func TestCRUDChangesAndStatePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	ctx := context.Background()
	expectChange := func(want bool) {
		t.Helper()
		select {
		case <-s.Changes():
			if !want {
				t.Fatal("unexpected change notification")
			}
		default:
			if want {
				t.Fatal("missing change notification")
			}
		}
	}
	task := model.Task{Title: "test", Channels: []notify.Channel{notify.ChannelSystem}, ActionExpression: json.RawMessage(`{}`)}
	task.ID, err = s.Add(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	expectChange(true)
	fired := time.Date(2026, 10, 15, 9, 0, 0, 123456000, time.UTC)
	if err := s.MarkLastFired(ctx, task.ID, fired); err != nil {
		t.Fatal(err)
	}
	expectChange(false)
	task.Title = "updated"
	task.LastFired = nil
	if err := s.Update(ctx, task); err != nil {
		t.Fatal(err)
	}
	expectChange(true)
	got, err := s.Get(ctx, task.ID)
	if err != nil || got.LastFired == nil || !got.LastFired.Equal(fired) {
		t.Fatalf("execution state overwritten: %+v %v", got, err)
	}
	// Multiple writes without a consumer must coalesce instead of blocking.
	done := make(chan error, 1)
	go func() {
		for range 3 {
			if err := s.Update(ctx, task); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("changes blocked CRUD")
	}
	expectChange(true)
	expectChange(false)
	if err := s.Remove(ctx, task.ID); err != nil {
		t.Fatal(err)
	}
	expectChange(true)
	if _, err := s.Get(ctx, task.ID); err == nil {
		t.Fatal("deleted task still exists")
	}
	if err := s.Remove(ctx, task.ID); err == nil {
		t.Fatal("missing delete accepted")
	}
	expectChange(false)
	if err := s.Update(ctx, task); err == nil {
		t.Fatal("missing update accepted")
	}
	expectChange(false)
}

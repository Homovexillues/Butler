package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
)

// Simulate the schema and encodings written by the former database/sql store.
func TestLegacyDatabaseCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE tasks (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL, body TEXT NOT NULL,
		schedule_type TEXT NOT NULL, schedule_expression TEXT NOT NULL,
		schedule_offset TEXT, action_type TEXT NOT NULL,
		action_expression TEXT NOT NULL, channel TEXT NOT NULL,
		enabled BOOL NOT NULL, last_fired DATETIME
	)`)
	if err != nil {
		t.Fatal(err)
	}
	fired := time.Date(2026, 10, 15, 9, 0, 0, 123456789, time.UTC)
	_, err = db.Exec(`INSERT INTO tasks VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy", "body", "solar", "03-05 09:00:00", `["T-0d"]`, "",
		`{"command":"echo"}`, []byte(`["mqtt"]`), true, fired.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	task, err := Get[model.Task](ctx, s, 1)
	if err != nil {
		t.Fatal(err)
	}
	if task.LastFired == nil || !task.LastFired.Equal(fired) ||
		string(task.ActionExpression) != `{"command":"echo"}` ||
		!reflect.DeepEqual(task.ScheduleOffset, []string{"T-0d"}) ||
		!reflect.DeepEqual(task.Channels, []notify.Channel{notify.ChannelMQTT}) {
		t.Fatalf("legacy task changed: %+v", task)
	}
	task.Body = ""
	task.Enabled = false
	if err := s.Update(ctx, task); err != nil {
		t.Fatal(err)
	}
	got, err := Get[model.Task](ctx, s, 1)
	if err != nil || got.Body != "" || got.Enabled || got.LastFired == nil || !got.LastFired.Equal(fired) {
		t.Fatalf("legacy update: %+v, %v", got, err)
	}
	var mode string
	var foreignKeys, timeout int
	if err := s.db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil || mode != "wal" {
		t.Fatalf("journal_mode=%q: %v", mode, err)
	}
	if err := s.db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys=%d: %v", foreignKeys, err)
	}
	if err := s.db.Raw("PRAGMA busy_timeout").Scan(&timeout).Error; err != nil || timeout != 5000 {
		t.Fatalf("busy_timeout=%d: %v", timeout, err)
	}
}

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
	err = Add(ctx, s, &want)
	if err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		got, err := Get[model.Task](ctx, s, want.ID)
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
		case typ := <-s.Changes():
			if !want {
				t.Fatal("unexpected change notification")
			}
			if typ != reflect.TypeFor[model.Task]() {
				t.Fatalf("unexpected change type: %v", typ)
			}
		default:
			if want {
				t.Fatal("missing change notification")
			}
		}
	}
	task := model.Task{Title: "test", Channels: []notify.Channel{notify.ChannelSystem}, ActionExpression: json.RawMessage(`{}`)}
	err = Add(ctx, s, &task)
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
	got, err := Get[model.Task](ctx, s, task.ID)
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
	if err := Remove[model.Task](ctx, s, task.ID); err != nil {
		t.Fatal(err)
	}
	expectChange(true)
	if _, err := Get[model.Task](ctx, s, task.ID); err == nil {
		t.Fatal("deleted task still exists")
	}
	if err := Remove[model.Task](ctx, s, task.ID); err == nil {
		t.Fatal("missing delete accepted")
	}
	expectChange(false)
	if err := s.Update(ctx, task); err == nil {
		t.Fatal("missing update accepted")
	}
	expectChange(false)
}

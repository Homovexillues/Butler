package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"butler/internal/action"
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
		ActionType:         action.ActionNotify,
		ActionExpression:   json.RawMessage(`{"channels":["mqtt"]}`),
		Channel:            notify.ChannelMQTT,
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
	want.Channel = notify.ChannelEmail
	want.Enabled = false
	if err := s.Update(ctx, want); err != nil {
		t.Fatal(err)
	}
	check()
}

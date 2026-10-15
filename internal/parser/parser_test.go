package parser

import (
	"butler/internal/action"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseJSONC(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		invalid   bool
	}{
		{"comments and trailing comma", "{\n// comment\n\"name\":\"butler\",\n}", false},
		{"empty", " \n", true}, {"invalid", "{ broken }", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.jsonc")
			if err := os.WriteFile(path, []byte(tc.raw), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := Parse[struct {
				Name string `json:"name"`
			}](path)
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid document accepted")
				}
				return
			}
			if err != nil || result.Name != "butler" {
				t.Fatalf("got %+v, %v", result, err)
			}
		})
	}
	if _, err := Parse[any](filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestMakeSchedule(t *testing.T) {
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		name       string
		kind       schedule.ScheduleType
		expression string
		offsets    []string
		want       time.Time
	}{
		{"once", schedule.ScheduleOnce, "2026-01-02 09:00:00", nil, time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)},
		{"solar", schedule.ScheduleSolar, "03-05 09:00:00", nil, time.Date(2026, 3, 5, 9, 0, 0, 0, time.Local)},
		{"cron", schedule.ScheduleCron, "0 9 * * *", nil, time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local)},
		{"interval", schedule.ScheduleInterval, "1h,30m", nil, since.Add(90 * time.Minute)},
		{"offset", schedule.ScheduleSolar, "03-05 09:00:00", []string{"T-1d"}, time.Date(2026, 3, 4, 9, 0, 0, 0, time.Local)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := MakeSchedule(tc.kind, tc.expression, tc.offsets)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := s.NextAfter(since)
			if !ok || !got.Equal(tc.want) {
				t.Fatalf("next=%v %v, want %v", got, ok, tc.want)
			}
		})
	}
	s, err := MakeSchedule(schedule.ScheduleLunar, "01-01 07:00:00", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.NextAfter(since); !ok {
		t.Fatal("lunar schedule has no next time")
	}
	for _, tc := range []struct {
		kind    schedule.ScheduleType
		expr    string
		offsets []string
	}{
		{"invalid", "", nil}, {schedule.ScheduleCron, "invalid", nil}, {schedule.ScheduleOnce, "not a date", nil}, {schedule.ScheduleSolar, "13-01 07:00:00", nil}, {schedule.ScheduleInterval, "", nil}, {schedule.ScheduleSolar, "03-05 09:00:00", []string{"Tbad"}},
	} {
		if _, err := MakeSchedule(tc.kind, tc.expr, tc.offsets); err == nil {
			t.Errorf("invalid schedule accepted: %+v", tc)
		}
	}
}

func TestMakeAction(t *testing.T) {
	if a, err := MakeAction("", ""); err != nil || a != nil {
		t.Fatalf("reminder action: %v %v", a, err)
	}
	a, err := MakeAction(action.ActionCommand, `{"Command":"example","Args":["a"],"Env":{"K":"V"}}`)
	if err != nil {
		t.Fatal(err)
	}
	command, ok := a.(action.CommandAction)
	if !ok || command.Command != "example" || len(command.Args) != 1 || command.Env["K"] != "V" {
		t.Fatalf("action=%+v", a)
	}
	if _, err := MakeAction(action.ActionCommand, "{"); err == nil {
		t.Fatal("malformed JSON accepted")
	}
	if _, err := MakeAction("unknown", "{}"); err == nil {
		t.Fatal("unknown action accepted")
	}
}

func validTask() model.Task {
	return model.Task{ID: 12, Title: "task", Body: "body", Channels: []notify.Channel{notify.ChannelSystem}, ScheduleType: schedule.ScheduleCron, ScheduleExpression: "0 9 * * *", ActionExpression: json.RawMessage(`{}`)}
}

func TestTaskToNodeAndValidation(t *testing.T) {
	task := validTask()
	fired := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	task.LastFired = &fired
	node, err := TaskToNode(task)
	if err != nil {
		t.Fatal(err)
	}
	if node.ID != task.ID || node.Title != task.Title || node.Body != task.Body || node.Action != nil || node.Schedule == nil || node.LastFired == nil || !node.LastFired.Equal(fired) || len(node.Channels) != 1 {
		t.Fatalf("node=%+v", node)
	}
	if err := ValidateTask(task); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*model.Task){func(t *model.Task) { t.Channels = []notify.Channel{""} }, func(t *model.Task) { t.ScheduleExpression = "bad" }, func(t *model.Task) { t.ActionType = "unknown" }} {
		bad := validTask()
		change(&bad)
		if _, err := TaskToNode(bad); err == nil {
			t.Fatal("invalid task converted")
		}
		if err := ValidateTask(bad); err == nil || !strings.Contains(err.Error(), "task 12") {
			t.Fatalf("validation missing task context: %v", err)
		}
	}
}

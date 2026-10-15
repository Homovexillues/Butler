package cli

import (
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTreeOrdersByNextTimeRegardlessOfLastFiredOrEnabled(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.Local)
	old := now.Add(-24 * time.Hour)
	tasks := []model.Task{
		{ID: 1, Title: "晚到任务", Enabled: true, ScheduleType: schedule.ScheduleCron, ScheduleExpression: "0 15 * * *", LastFired: &old},
		{ID: 2, Title: "早到任务", Enabled: false, ScheduleType: schedule.ScheduleCron, ScheduleExpression: "0 13 * * *"},
		{ID: 3, Title: "同时间任务", Enabled: true, ScheduleType: schedule.ScheduleCron, ScheduleExpression: "0 13 * * *"},
		{ID: 4, Title: "错误配置", Enabled: true, ScheduleType: "invalid"},
	}
	var out bytes.Buffer
	if err := printTasks(&out, tasks, now); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("table=%s", out.String())
	}
	for i, id := range []string{"2", "3", "1", "4"} {
		if strings.Fields(lines[i+1])[0] != id {
			t.Fatalf("wrong order: %s", out.String())
		}
	}
	if !strings.Contains(lines[1], "2026-10-15 13:00:00") || !strings.Contains(lines[3], "2026-10-15 15:00:00") {
		t.Fatalf("not calculated from now: %s", out.String())
	}
	if !strings.Contains(lines[4], "配置错误") {
		t.Fatalf("missing error row: %s", out.String())
	}
	if strings.Contains(out.String(), "已逾期") {
		t.Fatal("tree should not use execution history")
	}
}

func TestTreeEmpty(t *testing.T) {
	var out bytes.Buffer
	if err := printTasks(&out, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "暂无任务") {
		t.Fatal(out.String())
	}
}

func TestValidateTasksIncludesDisabledAndAggregates(t *testing.T) {
	good := model.Task{ID: 1, Title: "valid", Channels: []notify.Channel{notify.ChannelSystem}, ScheduleType: schedule.ScheduleCron, ScheduleExpression: "0 9 * * *", ActionExpression: json.RawMessage(`{}`)}
	if err := validateTasks([]model.Task{good}); err != nil {
		t.Fatal(err)
	}
	badSchedule := good
	badSchedule.ID = 2
	badSchedule.Title = "bad schedule"
	badSchedule.ScheduleExpression = "invalid"
	badChannel := good
	badChannel.ID = 3
	badChannel.Title = "bad channel"
	badChannel.Channels = nil
	badChannel.Enabled = false
	err := validateTasks([]model.Task{good, badSchedule, badChannel})
	if err == nil {
		t.Fatal("invalid tasks accepted")
	}
	for _, part := range []string{"task 2", "bad schedule", "task 3", "bad channel"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("missing %q: %v", part, err)
		}
	}
}

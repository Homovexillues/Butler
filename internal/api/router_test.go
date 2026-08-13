package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"butler/internal/model"
	"butler/internal/schedule"

	"github.com/gin-gonic/gin"
)

func TestHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(nil)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d,want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" {
		t.Errorf("status = %q,want %q", response.Status, "ok")
	}
}

func TestListTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	triggerAt := time.Now().Add(time.Hour)

	nodes := []*model.Node{
		{
			Title:    "测试任务",
			Body:     "测试正文",
			Channels: []string{"mqtt"},
			Schedule: schedule.Once{At: triggerAt},
		},
	}

	router := NewRouter(nodes)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response []taskResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response) != 1 {
		t.Fatalf("task count = %d, want 1", len(response))
	}

	task := response[0]

	if task.Title != "测试任务" {
		t.Errorf("title = %q, want %q", task.Title, "测试任务")
	}

	if task.Body != "测试正文" {
		t.Errorf("body = %q, want %q", task.Body, "测试正文")
	}

	if len(task.Channels) != 1 || task.Channels[0] != "mqtt" {
		t.Errorf("channels = %v, want [mqtt]", task.Channels)
	}

	if task.NextTriggeredAt == nil {
		t.Fatal("nextTriggeredAt is nil")
	}

	if !task.NextTriggeredAt.Equal(triggerAt) {
		t.Errorf(
			"nextTriggeredAt = %v, want %v",
			task.NextTriggeredAt,
			triggerAt,
		)
	}
}

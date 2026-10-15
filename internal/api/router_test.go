package api

import (
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
	"butler/internal/store"
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func testRouter(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s, err := store.NewStore(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return NewRouter(s), s
}
func apiCall(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
func fixtureTask() model.Task {
	return model.Task{Title: "测试任务", Body: "测试正文", Enabled: true, Channels: []notify.Channel{notify.ChannelSystem}, ScheduleType: schedule.ScheduleInterval, ScheduleExpression: "1h", ActionExpression: json.RawMessage(`{}`)}
}

func TestPublicRoutes(t *testing.T) {
	router, _ := testRouter(t)
	for _, path := range []string{"/", "/ping"} {
		response := apiCall(t, router, http.MethodGet, path, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
		if path == "/ping" && response.Body.String() != "pong" {
			t.Fatal(response.Body.String())
		}
	}
}
func TestTaskCRUD(t *testing.T) {
	router, s := testRouter(t)
	task := fixtureTask()
	response := apiCall(t, router, http.MethodPost, "/api/v1/add", task)
	if response.Code != http.StatusOK {
		t.Fatalf("add: %d %s", response.Code, response.Body.String())
	}
	var added struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &added); err != nil {
		t.Fatal(err)
	}
	if added.ID <= 0 {
		t.Fatal("no generated ID")
	}
	task.ID = added.ID
	read := func() model.Task {
		t.Helper()
		response := apiCall(t, router, http.MethodPost, "/api/v1/get", map[string]int64{"id": task.ID})
		if response.Code != http.StatusOK {
			t.Fatalf("get: %d %s", response.Code, response.Body.String())
		}
		var got model.Task
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := read(); !reflect.DeepEqual(got, task) {
		t.Fatalf("got %+v, want %+v", got, task)
	}
	task.Title = "修改后"
	task.Body = ""
	task.Enabled = false
	task.Channels = []notify.Channel{notify.ChannelEmail}
	response = apiCall(t, router, http.MethodPost, "/api/v1/update", task)
	if response.Code != http.StatusOK {
		t.Fatalf("update: %d %s", response.Code, response.Body.String())
	}
	if got := read(); !reflect.DeepEqual(got, task) {
		t.Fatalf("update got %+v, want %+v", got, task)
	}
	response = apiCall(t, router, http.MethodPost, "/api/v1/delete", map[string]int64{"id": task.ID})
	if response.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", response.Code, response.Body.String())
	}
	rows, err := s.GetAll(context.Background())
	if err != nil || len(rows) != 0 {
		t.Fatalf("deleted row remains: %+v %v", rows, err)
	}
	response = apiCall(t, router, http.MethodPost, "/api/v1/get", map[string]int64{"id": task.ID})
	if response.Code < 400 {
		t.Fatal("deleted task returned success")
	}
}

func TestListTasks(t *testing.T) {
	router, s := testRouter(t)
	empty := apiCall(t, router, http.MethodGet, "/api/v1/tasks", nil)
	if empty.Code != http.StatusOK || empty.Body.String() != "[]" {
		t.Fatalf("empty list: %d %s", empty.Code, empty.Body.String())
	}
	task := fixtureTask()
	fired := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	task.LastFired = &fired
	if _, err := s.Add(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	response := apiCall(t, router, http.MethodGet, "/api/v1/tasks", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("list: %d %s", response.Code, response.Body.String())
	}
	var rows []taskResponse
	if err := json.Unmarshal(response.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%+v", rows)
	}
	got := rows[0]
	if got.Title != task.Title || got.Body != task.Body || !reflect.DeepEqual(got.Channels, task.Channels) || got.LastFired == nil || !got.LastFired.Equal(fired) || got.NextTriggeredAt == nil || !got.NextTriggeredAt.Equal(fired.Add(time.Hour)) {
		t.Fatalf("response=%+v", got)
	}
}

func TestHandlersRejectMalformedJSON(t *testing.T) {
	router, _ := testRouter(t)
	for _, endpoint := range []string{"add", "update", "get", "delete"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/"+endpoint, bytes.NewBufferString("{"))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", endpoint, response.Code)
		}
	}
}

func TestWritesRejectInvalidTaskWithoutPersisting(t *testing.T) {
	router, s := testRouter(t)
	for _, endpoint := range []string{"add", "update"} {
		for _, change := range []func(*model.Task){func(t *model.Task) { t.ScheduleExpression = "bad" }, func(t *model.Task) { t.ActionType = "unknown" }, func(t *model.Task) { t.Channels = nil }} {
			task := fixtureTask()
			change(&task)
			response := apiCall(t, router, http.MethodPost, "/api/v1/"+endpoint, task)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("%s: %d %s", endpoint, response.Code, response.Body.String())
			}
		}
	}
	rows, err := s.GetAll(context.Background())
	if err != nil || len(rows) != 0 {
		t.Fatalf("invalid rows persisted: %+v %v", rows, err)
	}
}

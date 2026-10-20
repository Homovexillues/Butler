package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"butler/internal/config"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"
	"butler/internal/store"

	"github.com/gin-gonic/gin"
	gomail "github.com/wneessen/go-mail"
)

func TestEmailSenderNameAndAddress(t *testing.T) {
	router, s := testRouter(t)
	for _, name := range []string{"顾霈圭", `管家 "提醒" \\ Butler`, ""} {
		body := map[string]any{
			"host": "smtp.example.test", "port": 465, "username": "sender@example.test",
			"authcode": "secret", "fromName": name, "fromAddress": "sender@example.test",
			"to": []string{"recipient@example.test"},
		}
		response := apiCall(t, router, http.MethodPost, "/api/v1/settings/email", body)
		if response.Code != http.StatusOK {
			t.Fatal(response.Body.String())
		}
		cfg, err := config.LoadConfig(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		from, err := mail.ParseAddress(cfg.Email.From)
		if err != nil || from.Name != name || from.Address != "sender@example.test" {
			t.Fatalf("sender lost: %q, %v", cfg.Email.From, err)
		}
		msg := gomail.NewMsg()
		if err := msg.From(cfg.Email.From); err != nil {
			t.Fatal(err)
		}
		if got := msg.GetFrom(); len(got) != 1 || got[0].Name != name {
			t.Fatalf("mail sender name lost: %+v", got)
		}
		response = apiCall(t, router, http.MethodGet, "/api/v1/settings/email", nil)
		var result struct {
			FromName    string `json:"fromName"`
			FromAddress string `json:"fromAddress"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.FromName != name || result.FromAddress != "sender@example.test" {
			t.Fatalf("sender fields lost: %+v", result)
		}
	}
}

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
	rows, err := store.GetAll[model.Task](context.Background(), s)
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
	if err := store.Add(context.Background(), s, &task); err != nil {
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
	rows, err := store.GetAll[model.Task](context.Background(), s)
	if err != nil || len(rows) != 0 {
		t.Fatalf("invalid rows persisted: %+v %v", rows, err)
	}
}

// Management must remain usable when old tasks have expired or invalid schedules.
func TestManagementListIncludesEditableTasks(t *testing.T) {
	router, s := testRouter(t)
	fixtures := []model.Task{fixtureTask(), fixtureTask(), fixtureTask()}
	fixtures[0].Title = "never fired"
	fixtures[1].Title = "expired"
	fixtures[1].ScheduleType = schedule.ScheduleOnce
	fixtures[1].ScheduleExpression = "2000-01-01 00:00:00"
	fixtures[2].Title = "invalid legacy schedule"
	fixtures[2].ScheduleExpression = "invalid"
	for i := range fixtures {
		if err := store.Add(context.Background(), s, &fixtures[i]); err != nil {
			t.Fatal(err)
		}
	}
	response := apiCall(t, router, http.MethodGet, "/api/v1/tasks", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("list: %d %s", response.Code, response.Body.String())
	}
	var rows []taskResponse
	if err := json.Unmarshal(response.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows: %+v", rows)
	}
	for i, row := range rows {
		if !reflect.DeepEqual(row.Task, fixtures[i]) {
			t.Fatalf("editable data lost: %+v", row.Task)
		}
	}
	if rows[0].NextTriggeredAt == nil || rows[0].NextTriggeredAt.IsZero() {
		t.Fatal("never-fired task has no estimate")
	}
	if rows[1].NextTriggeredAt != nil {
		t.Fatal("expired task has a next trigger")
	}
	if rows[2].ScheduleError == "" {
		t.Fatal("invalid schedule has no explanation")
	}
}

func TestDuplicateAddReportsFailure(t *testing.T) {
	router, _ := testRouter(t)
	task := fixtureTask()
	first := apiCall(t, router, http.MethodPost, "/api/v1/add", task)
	if first.Code != http.StatusOK {
		t.Fatal(first.Body.String())
	}
	second := apiCall(t, router, http.MethodPost, "/api/v1/add", task)
	if second.Code < 400 {
		t.Fatalf("duplicate reported success: %s", second.Body.String())
	}
}

func TestSettingsRestartRequiredSinceLoad(t *testing.T) {
	_, s := testRouter(t)
	loadedAt := time.Now().UTC()
	router := NewRouter(s, loadedAt)
	check := func(router http.Handler, path string, want bool) time.Time {
		t.Helper()
		response := apiCall(t, router, http.MethodGet, "/api/v1/settings/"+path, nil)
		var result struct {
			Settings struct {
				UpdatedAt time.Time `json:"updatedAt"`
			} `json:"settings"`
			ConfigLoadedAt  time.Time `json:"configLoadedAt"`
			RestartRequired bool      `json:"restartRequired"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || result.RestartRequired != want || result.ConfigLoadedAt.IsZero() {
			t.Fatalf("%s restart status: %s", path, response.Body.String())
		}
		return result.Settings.UpdatedAt
	}
	for _, tc := range []struct {
		path  string
		value any
	}{
		{"mqtt", notify.MqttSetting{Address: "127.0.0.1:1883", Topic: "test", Username: "u", Password: "secret", SkipVerify: true, UpdatedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
		{"email", notify.EmailNotifier{Host: "smtp.example.test", Port: 465, Username: "u", Authcode: "secret", From: "from@example.test", To: []string{"to@example.test"}, UpdatedAt: time.Now().Add(24 * time.Hour)}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			check(router, tc.path, false)
			for range 2 {
				response := apiCall(t, router, http.MethodPost, "/api/v1/settings/"+tc.path, tc.value)
				if response.Code != http.StatusOK {
					t.Fatal(response.Body.String())
				}
				updatedAt := check(router, tc.path, true)
				if !updatedAt.After(loadedAt) || updatedAt.After(time.Now()) {
					t.Fatal("modification time must be server generated")
				}
				// Reopening a fresh router represents a new configuration load.
				newRouter := NewRouter(s, time.Now().UTC())
				check(newRouter, tc.path, false)
				// Reading again through the running router must retain the warning.
				check(router, tc.path, true)
				loadedAt = time.Now().UTC()
				router = NewRouter(s, loadedAt)
			}
		})
	}
}

func TestChannelSettingsSaveAndRead(t *testing.T) {
	router, s := testRouter(t)
	// A live broker may already own this port; saving must not start a broker.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	mqtt := notify.MqttSetting{ID: 99, Address: listener.Addr().String(), Topic: "test", Username: "user", Password: "mqtt-secret", SkipVerify: true}
	email := notify.EmailNotifier{ID: 99, Host: "smtp.example.test", Port: 465, Username: "user", Authcode: "email-secret", From: "from@example.test", To: []string{"to@example.test"}}
	for _, tc := range []struct {
		path   string
		value  any
		secret string
	}{
		{"mqtt", mqtt, mqtt.Password}, {"email", email, email.Authcode},
	} {
		t.Run(tc.path, func(t *testing.T) {
			empty := apiCall(t, router, http.MethodGet, "/api/v1/settings/"+tc.path, nil)
			if empty.Code != http.StatusOK {
				t.Fatalf("empty settings: %s", empty.Body.String())
			}
			for range 2 {
				response := apiCall(t, router, http.MethodPost, "/api/v1/settings/"+tc.path, tc.value)
				if response.Code != http.StatusOK {
					t.Fatalf("save: %d %s", response.Code, response.Body.String())
				}
				if strings.Contains(response.Body.String(), tc.secret) {
					t.Fatal("save exposed password")
				}
			}
			response := apiCall(t, router, http.MethodGet, "/api/v1/settings/"+tc.path, nil)
			var got struct {
				Settings    map[string]any `json:"settings"`
				HasPassword bool           `json:"hasPassword"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || !got.HasPassword || got.Settings["id"] != float64(config.DefaultSettingID) || strings.Contains(response.Body.String(), tc.secret) {
				t.Fatalf("read exposed secret or wrong singleton: %s", response.Body.String())
			}
		})
	}
	// Blank secrets keep the saved values; explicit zero values still update.
	mqtt.Password = ""
	mqtt.ClientID = "changed"
	email.Authcode = ""
	email.To = []string{"new@example.test"}
	for path, value := range map[string]any{"mqtt": mqtt, "email": email} {
		response := apiCall(t, router, http.MethodPost, "/api/v1/settings/"+path, value)
		if response.Code != http.StatusOK {
			t.Fatal(response.Body.String())
		}
	}
	cfg, err := config.LoadConfig(context.Background(), s)
	if err != nil || cfg.Mqtt.Password != "mqtt-secret" || cfg.Mqtt.ClientID != "changed" || cfg.Email.Authcode != "email-secret" || len(cfg.Email.To) != 1 || cfg.Email.To[0] != "new@example.test" {
		t.Fatal("saved settings or preserved passwords do not match")
	}
	rows, err := store.GetAll[notify.MqttSetting](context.Background(), s)
	if err != nil || len(rows) != 1 {
		t.Fatalf("MQTT singleton count=%d, %v", len(rows), err)
	}
	emails, err := store.GetAll[notify.EmailNotifier](context.Background(), s)
	if err != nil || len(emails) != 1 {
		t.Fatalf("email singleton count=%d, %v", len(emails), err)
	}
	select {
	case <-s.Changes():
		t.Fatal("settings must not notify task scheduler")
	default:
	}
}

func TestChannelSettingsRejectInvalidWithoutWriting(t *testing.T) {
	router, s := testRouter(t)
	for _, tc := range []struct {
		path string
		body any
	}{
		{"mqtt", map[string]any{}},
		{"mqtt", notify.MqttSetting{Address: "bad-address", Topic: "test", Username: "u", Password: "p", SkipVerify: true}},
		{"mqtt", notify.MqttSetting{Address: "127.0.0.1:1883", Topic: "test", Username: "u", Password: "p", CertFile: "missing", KeyFile: "missing"}},
		{"email", map[string]any{}},
		{"email", notify.EmailNotifier{Host: "smtp.example.test", Port: 465, Username: "u", Authcode: "p", From: "bad-address", To: []string{"to@example.test"}}},
	} {
		response := apiCall(t, router, http.MethodPost, "/api/v1/settings/"+tc.path, tc.body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", tc.path, response.Code, response.Body.String())
		}
	}
	mqtt, err := store.GetAll[notify.MqttSetting](context.Background(), s)
	if err != nil || len(mqtt) != 0 {
		t.Fatal("invalid MQTT settings persisted")
	}
	email, err := store.GetAll[notify.EmailNotifier](context.Background(), s)
	if err != nil || len(email) != 0 {
		t.Fatal("invalid email settings persisted")
	}
}

func TestRestartEndpointRemoved(t *testing.T) {
	router, _ := testRouter(t)
	response := apiCall(t, router, http.MethodPost, "/api/v1/settings/restart", nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("removed restart endpoint: %d", response.Code)
	}
}

func TestActionsAndSettingsTransferRoutes(t *testing.T) {
	router, _ := testRouter(t)
	response := apiCall(t, router, http.MethodGet, "/api/v1/actions", nil)
	if response.Code != http.StatusOK || response.Body.String() != `["command"]` {
		t.Fatalf("actions: %s", response.Body.String())
	}
	response = apiCall(t, router, http.MethodGet, "/api/v1/settings/output", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("export: %s", response.Body.String())
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/settings/load", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("import route: %d", recorder.Code)
	}
}

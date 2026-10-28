package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"butler/internal/action"
	"butler/internal/config"
	"butler/internal/engine"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/parser"
	"butler/internal/store"

	"github.com/gin-gonic/gin"
)

func addTaskHandler(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var task model.Task
		if err := c.ShouldBindBodyWithJSON(&task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		if err := parser.ValidateTask(task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		task.LastAttempt = nil
		task.LastFired = nil
		err := store.Add(c.Request.Context(), s, &task)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": task.ID})
	}
}

func deleteTaskHandler(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID int64 `json:"id"`
		}
		if err := c.ShouldBindBodyWithJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		if err := store.Remove[model.Task](c.Request.Context(), s, req.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": req.ID})
	}
}

func listTaskHandler(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskRows, err := store.GetAll[model.Task](c.Request.Context(), s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		tasks := make([]taskResponse, 0, len(taskRows))
		now := time.Now()
		for _, taskRow := range taskRows {
			task := taskResponse{Task: taskRow, Channels: append([]notify.Channel{}, taskRow.Channels...)}
			taskSchedule, err := parser.MakeSchedule(taskRow.ScheduleType, taskRow.ScheduleExpression, taskRow.ScheduleOffset)
			if err != nil {
				task.ScheduleError = err.Error()
			} else if taskRow.Enabled {
				since := engine.Since(timeValue(taskRow.LastAttempt), timeValue(taskRow.LastFired), now)
				if next, ok := taskSchedule.NextAfter(since); ok {
					task.NextTriggeredAt = &next
				}
			}
			tasks = append(tasks, task)
		}
		c.JSON(http.StatusOK, tasks)
	}
}

func timeValue(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func updateTaskHandler(store *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var task model.Task
		if err := c.ShouldBindBodyWithJSON(&task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		if err := parser.ValidateTask(task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		err := store.UpdateTask(c.Request.Context(), task)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, "ok")
	}
}

func getTaskHandler(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID int64 `json:"id"`
		}
		if err := c.ShouldBindBodyWithJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		task, err := store.Get[model.Task](c.Request.Context(), s, req.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, task)
	}
}

func getChannelsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, notify.AllChannels())
	}
}

func getActionsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, action.AllActionTypes())
	}
}

func outputTasksHandler(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks, err := store.GetAll[model.Task](c.Request.Context(), s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		data, err := json.MarshalIndent(tasks, "", "    ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", `attachment; filename="tasks.json"`)
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	}
}

func loadTasksHandler(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 5<<20)
		header, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		file, err := header.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		data, readErr := io.ReadAll(file)
		err = errors.Join(readErr, file.Close())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var tasks []model.Task
		if err := json.Unmarshal(data, &tasks); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		for _, task := range tasks {
			if err := parser.ValidateTask(task); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			task.LastAttempt = nil
			task.LastFired = nil
			if err := store.Add(c.Request.Context(), s, &task); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.String(http.StatusOK, "succeed")
	}
}

func loginHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
	}
}

func identityHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"username": "homovexillues", "authenticated": true})
	}
}

func getMqttConfigHandler(s *store.Store, loadedAt time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := config.LoadConfig(c.Request.Context(), s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		hasPassword := cfg.Mqtt.Password != ""
		cfg.Mqtt.Password = ""
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{
			"settings": cfg.Mqtt, "hasPassword": hasPassword,
			"configLoadedAt": loadedAt, "restartRequired": cfg.Mqtt.UpdatedAt.After(loadedAt),
		})
	}
}

func getEmailConfigHandler(s *store.Store, loadedAt time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := config.LoadConfig(c.Request.Context(), s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		hasPassword := cfg.Email.Authcode != ""
		cfg.Email.Authcode = ""
		fromName, fromAddress := "", cfg.Email.From
		if from, err := mail.ParseAddress(cfg.Email.From); err == nil {
			fromName, fromAddress = from.Name, from.Address
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{
			"settings": cfg.Email, "hasPassword": hasPassword,
			"fromName": fromName, "fromAddress": fromAddress,
			"configLoadedAt": loadedAt, "restartRequired": cfg.Email.UpdatedAt.After(loadedAt),
		})
	}
}

func configMqttHandler(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var value notify.MqttSetting
		if err := c.ShouldBindJSON(&value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg, err := config.LoadConfig(c.Request.Context(), s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		value.ID = config.DefaultSettingID
		// Modification time is controlled by GORM, never by the request body.
		value.UpdatedAt = time.Time{}
		if value.Password == "" {
			value.Password = cfg.Mqtt.Password
		}
		if err := value.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.Upsert(c.Request.Context(), s, &value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"restartRequired": true})
	}
}

func configEmailHandler(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			notify.EmailNotifier
			FromName    *string `json:"fromName"`
			FromAddress *string `json:"fromAddress"`
		}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		value := request.EmailNotifier
		if request.FromName != nil || request.FromAddress != nil {
			if request.FromAddress == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请填写发件邮箱"})
				return
			}
			address, err := mail.ParseAddress(strings.TrimSpace(*request.FromAddress))
			if err != nil || address.Name != "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "发件邮箱请仅填写邮箱地址，姓名填写在发件人姓名中"})
				return
			}
			if request.FromName != nil {
				address.Name = strings.TrimSpace(*request.FromName)
			}
			if strings.ContainsAny(address.Name, "\r\n") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "发件人姓名不能包含换行"})
				return
			}
			value.From = address.String()
		}
		cfg, err := config.LoadConfig(c.Request.Context(), s)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		value.ID = config.DefaultSettingID
		value.UpdatedAt = time.Time{}
		if value.Authcode == "" {
			value.Authcode = cfg.Email.Authcode
		}
		_, err = notify.NewEmailNotifier(value.Host, value.Port, value.Username, value.Authcode, value.From, value.To)
		if err == nil && value.Port > 65535 {
			err = errors.New("email: port must not exceed 65535")
		}
		if err == nil {
			for _, address := range append([]string{value.From}, value.To...) {
				if _, err = mail.ParseAddress(address); err != nil {
					break
				}
			}
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.Upsert(c.Request.Context(), s, &value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"restartRequired": true})
	}
}

package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"butler/internal/action"
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
				since := now
				if taskRow.LastFired != nil {
					since = *taskRow.LastFired
				}
				if next, ok := taskSchedule.NextAfter(since); ok {
					task.NextTriggeredAt = &next
				}
			}
			tasks = append(tasks, task)
		}
		c.JSON(http.StatusOK, tasks)
	}
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
		err := store.Update(c.Request.Context(), task)
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
		c.Header("Content-Disposition", `atachment; filename="tasks.json"`)
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
			if err := store.Add(c.Request.Context(), s, &task); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.String(http.StatusOK, "succeed")
	}
}

func loginHandler() gin.HandlerFunc {
	return nil
}

func identityHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"username": "homovexillues", "authenticated": true})
	}
}

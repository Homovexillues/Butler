package api

import (
	"net/http"
	"time"

	"butler/internal/action"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/parser"
	"butler/internal/store"

	"github.com/gin-gonic/gin"
)

func addTaskHandler(store *store.Store) gin.HandlerFunc {
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
		id, err := store.Add(c.Request.Context(), task)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id})
	}
}

func deleteTaskHandler(store *store.Store) gin.HandlerFunc {
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
		if err := store.Remove(c.Request.Context(), req.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": req.ID})
	}
}

func listTaskHandler(store *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskRows, err := store.GetAll(c.Request.Context())
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

func getTaskHandler(store *store.Store) gin.HandlerFunc {
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
		task, err := store.Get(c.Request.Context(), req.ID)
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

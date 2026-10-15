package api

import (
	"net/http"
	"time"

	"butler/internal/model"
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
			c.JSON(http.StatusBadRequest, err)
			return
		}
		tasks := make([]taskResponse, 0, len(taskRows))
		for _, taskRow := range taskRows {
			taskSchedule, err := parser.MakeSchedule(taskRow.ScheduleType, taskRow.ScheduleExpression, taskRow.ScheduleOffset)
			if err != nil {
				c.JSON(http.StatusBadRequest, err)
				return
			}
			var nextTriggered time.Time
			if taskRow.LastFired != nil {
				var ok bool
				nextTriggered, ok = taskSchedule.NextAfter(*taskRow.LastFired)
				if !ok {
					c.JSON(http.StatusBadRequest, err)
					return
				}
			}
			task := taskResponse{
				Title:           taskRow.Title,
				Body:            taskRow.Body,
				Channels:        taskRow.Channels,
				LastFired:       taskRow.LastFired,
				NextTriggeredAt: &nextTriggered,
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

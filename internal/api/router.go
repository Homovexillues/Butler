// Pakcage api is an api router pakcage
package api

import (
	"embed"
	"net/http"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/parser"
	"butler/internal/store"

	"github.com/gin-gonic/gin"
)

//go:embed web
var webFiles embed.FS

type taskResponse struct {
	Title           string           `json:"title"`
	Body            string           `json:"body"`
	Channels        []notify.Channel `json:"channels"`
	LastFired       *time.Time       `json:"lastFired"`
	NextTriggeredAt *time.Time       `json:"nextTriggeredAt"`
}

func NewRouter(store *store.Store) *gin.Engine {
	router := gin.Default()
	router.GET("/", func(c *gin.Context) {
		page, err := webFiles.ReadFile("web/index.html")
		if err != nil {
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})
	api := router.Group("/api/v1")
	api.GET("/tasks", func(c *gin.Context) {
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
	})
	api.POST("/add", func(c *gin.Context) {
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
		id, err := store.Add(c, task)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id})
	})
	api.POST("/delete", func(c *gin.Context) {
		var req struct {
			ID int64 `json:"id"`
		}
		if err := c.ShouldBindBodyWithJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		if err := store.Remove(c, req.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": req.ID})
	})
	api.POST("/update", func(c *gin.Context) {
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
		err := store.Update(c, task)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, "ok")
	})
	api.POST("/get", func(c *gin.Context) {
		var req struct {
			ID int64 `json:"id"`
		}
		if err := c.ShouldBindBodyWithJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		task, err := store.Get(c, req.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, task)
	})
	return router
}

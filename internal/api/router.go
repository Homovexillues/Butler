// Pakcage api is an api router pakcage
package api

import (
	"net/http"
	"time"

	"butler/internal/store"

	"github.com/gin-gonic/gin"
)

type taskResponse struct {
	Title           string     `json:"title"`
	Body            string     `json:"body"`
	Channels        []string   `json:"channels"`
	NextTriggeredAt *time.Time `json:"nextTriggeredAt"`
}

func NewRouter(store *store.Store) *gin.Engine {
	router := gin.Default()
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
		}
		tasks := make([]taskResponse, 0, len(taskRows))
		for _, taskRow := range taskRows {
			task := taskResponse{
				Title: taskRow.Title,
				Body:  taskRow.Body,
			}
			tasks = append(tasks, task)
		}
		c.JSON(http.StatusOK, tasks)
	})
	return router
}

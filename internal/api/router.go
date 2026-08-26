// Pakcage api is an api router pakcage
package api

import (
	"net/http"
	"time"

	"butler/internal/model"

	"github.com/gin-gonic/gin"
)

type taskResponse struct {
	Title           string     `json:"title"`
	Body            string     `json:"body"`
	Channels        []string   `json:"channels"`
	NextTriggeredAt *time.Time `json:"nextTriggeredAt"`
}

func NewRouter(nodes []*model.Node) *gin.Engine {
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
		tasks := make([]taskResponse, 0, len(nodes))
		now := time.Now()
		for _, node := range nodes {
			task := taskResponse{
				Title: node.Title,
				Body:  node.Body,
			}
			if next, found := node.Schedule.NextAfter(now); found {
				task.NextTriggeredAt = &next
			}
			tasks = append(tasks, task)
		}
		c.JSON(http.StatusOK, tasks)
	})
	return router
}

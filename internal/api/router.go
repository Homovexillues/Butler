// Package api serves the management API and embedded web page.
package api

import (
	"embed"
	"net/http"
	"time"

	"butler/internal/notify"
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
			c.String(http.StatusInternalServerError, "failed to load page: %v", err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	api := router.Group("/api/v1")
	protected := api.Group("")
	protected.GET("/tasks", listTaskHandler(store))
	protected.POST("/add", addTaskHandler(store))
	protected.POST("/delete", deleteTaskHandler(store))
	protected.POST("/update", updateTaskHandler(store))
	protected.POST("/get", getTaskHandler(store))
	return router
}

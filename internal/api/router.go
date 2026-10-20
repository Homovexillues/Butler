// Package api serves the management API and embedded web page.
package api

import (
	"embed"
	"net/http"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/store"

	"github.com/gin-gonic/gin"
)

//go:embed web
var webFiles embed.FS

type taskResponse struct {
	model.Task
	Channels        []notify.Channel `json:"channels"`
	NextTriggeredAt *time.Time       `json:"nextTriggeredAt"`
	ScheduleError   string           `json:"scheduleError,omitempty"`
}

// Production passes the time immediately before loading runtime configuration.
// Standalone routers use their creation time as the baseline.
func NewRouter(store *store.Store, loadedAt ...time.Time) *gin.Engine {
	configLoadedAt := time.Now().UTC()
	if len(loadedAt) > 0 {
		configLoadedAt = loadedAt[0]
	}
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
	auth := api.Group("/api/v1/auth")
	{

		auth.GET("whoami", identityHandler())
		auth.POST("login", loginHandler())
	}
	protected := api.Group("")
	{

		protected.GET("/tasks", listTaskHandler(store))
		protected.POST("/add", addTaskHandler(store))
		protected.POST("/delete", deleteTaskHandler(store))
		protected.POST("/update", updateTaskHandler(store))
		protected.POST("/get", getTaskHandler(store))
		protected.GET("/channels", getChannelsHandler())
		protected.GET("/actions", getActionsHandler())
	}
	settings := protected.Group("/settings")
	{
		settings.GET("/mqtt", getMqttConfigHandler(store, configLoadedAt))
		settings.GET("/email", getEmailConfigHandler(store, configLoadedAt))
		settings.GET("/output", outputTasksHandler(store))
		settings.POST("/load", loadTasksHandler(store))
		settings.POST("/mqtt", configMqttHandler(store))
		settings.POST("/email", configEmailHandler(store))
	}
	return router
}

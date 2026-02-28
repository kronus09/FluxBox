package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine) {
	// API 组
	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/status", GetStatus)
		apiGroup.POST("/add", AddSource)
		apiGroup.DELETE("/delete/:id", DeleteSource)
		apiGroup.POST("/aggregate", TriggerAggregate)
	}
}

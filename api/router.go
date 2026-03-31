package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine) {
	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/status", GetStatus)
		apiGroup.POST("/add", AddSource)
		apiGroup.DELETE("/delete/:id", DeleteSource)
		apiGroup.POST("/update", UpdateSource)
		apiGroup.POST("/aggregate", TriggerAggregate)
		apiGroup.GET("/test/:id", TestSource)
		apiGroup.POST("/test", TestSources)
		apiGroup.GET("/filter-words", GetFilterWords)
		apiGroup.POST("/filter-words", AddFilterWord)
		apiGroup.DELETE("/filter-words/:word", DeleteFilterWord)
		apiGroup.POST("/filter-words/reset", ResetFilterWords)
	}
}

package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine) {
	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/status", GetStatus)
		apiGroup.POST("/add", AddSource)
		apiGroup.DELETE("/delete/:id", DeleteSource)
		apiGroup.POST("/update", UpdateSource)
		apiGroup.POST("/toggle/:id", ToggleSource)
		apiGroup.POST("/aggregate", TriggerAggregate)
		apiGroup.GET("/test/:id", TestSource)
		apiGroup.POST("/test", TestSources)
		apiGroup.GET("/health/:id", CheckSingleHealth)
		apiGroup.POST("/health", CheckAllHealth)
		apiGroup.GET("/filter-words", GetFilterWords)
		apiGroup.POST("/filter-words", AddFilterWord)
		apiGroup.DELETE("/filter-words/:word", DeleteFilterWord)
		apiGroup.POST("/filter-words/reset", ResetFilterWords)
		apiGroup.GET("/schedule", GetSchedule)
		apiGroup.POST("/schedule", SaveSchedule)
		apiGroup.POST("/generate-multi", GenerateMultiConfig)
		apiGroup.GET("/multi-config", GetMultiConfig)
		apiGroup.GET("/multi-status", GetMultiConfigStatus)
		apiGroup.GET("/home-menu-sources", GetHomeMenuSources)
		apiGroup.POST("/home-menu-source", SetHomeMenuSource)
		apiGroup.GET("/global-config", GetGlobalConfig)
		apiGroup.POST("/global-config", SaveGlobalConfig)
		apiGroup.POST("/localize/:id", LocalizeSourceHandler)
		apiGroup.POST("/localize", BatchLocalizeHandler)
		apiGroup.GET("/sources/export", ExportSources)
		apiGroup.POST("/sources/import", ImportSources)
	}
}

package main

import (
	"FluxBox/api"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 初始化数据
	api.InitData()

	// 2. 设置 Gin
	r := gin.Default()

	// 3. 托管静态文件 (web 目录)
	r.Static("/ui", "./web")
	r.Static("/static", "./web/static")
	// 根路径自动跳转到 UI
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/ui/")
	})

	// 4. 影视Box 聚合接口 (持久化读取)
	r.GET("/config", func(c *gin.Context) {
		api.Mu.Lock()
		config := api.MemoryConfig
		api.Mu.Unlock()
		c.JSON(200, config)
	})

	// 5. 注册 API 路由
	api.RegisterRoutes(r)

	// 6. 启动
	println("FluxBox 启动成功，访问地址: http://localhost:20504")
	r.Run(":20504")
}

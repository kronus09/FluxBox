package main

import (
	"FluxBox/api"
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

func getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	if c.GetHeader("X-Forwarded-Ssl") == "on" {
		scheme = "https"
	}
	host := c.Request.Host
	if host == "" {
		host = "localhost:20504"
	}
	return scheme + "://" + host
}

func replaceHostInConfig(configJSON []byte, baseURL string) []byte {
	return bytes.ReplaceAll(configJSON, []byte("__HOST__"), []byte(baseURL))
}

func main() {
	api.InitData()

	r := gin.Default()
	r.SetTrustedProxies(nil)

	r.Static("/ui", "./web")
	r.Static("/static", "./web/static")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/ui/")
	})

	r.GET("/config", func(c *gin.Context) {
		baseURL := getBaseURL(c)
		api.Mu.Lock()
		config := api.MemoryConfig
		api.Mu.Unlock()
		
		configJSON, _ := json.Marshal(config)
		configJSON = replaceHostInConfig(configJSON, baseURL)
		
		c.Data(200, "application/json", configJSON)
	})

	r.GET("/multi-config", func(c *gin.Context) {
		baseURL := getBaseURL(c)
		api.Mu.Lock()
		config := api.MemoryMultiConfig
		api.Mu.Unlock()
		
		if len(config.VideoList) == 0 {
			c.JSON(200, map[string]interface{}{
				"urls":      []interface{}{},
				"videoList": []interface{}{},
			})
			return
		}
		
		result := map[string]interface{}{
			"urls":      config.VideoList,
			"videoList": config.VideoList,
		}
		resultJSON, _ := json.Marshal(result)
		resultJSON = replaceHostInConfig(resultJSON, baseURL)
		
		c.Data(200, "application/json", resultJSON)
	})

	r.GET("/home", api.HandleHomeAPI)

	r.GET("/source/:id", api.HandleSourceConfig)

	api.RegisterRoutes(r)

	println("FluxBox 启动成功，访问地址: http://localhost:20504")
	r.Run(":20504")
}

package main

import (
	"FluxBox/api"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	r.Static("/jar", "./data/jars")
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
		
		c.Data(200, "application/json; charset=utf-8", configJSON)
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
		
		c.Data(200, "application/json; charset=utf-8", resultJSON)
	})

	r.GET("/home", api.HandleHomeAPI)

	r.GET("/source/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "无效的源ID"})
			return
		}

		cachePath := fmt.Sprintf("data/sources/%d.json", id)
		data, err := os.ReadFile(cachePath)
		if err != nil {
			c.JSON(404, gin.H{"error": "配置不存在或未缓存"})
			return
		}

		baseURL := getBaseURL(c)
		data = replaceHostInConfig(data, baseURL)
		c.Data(200, "application/json; charset=utf-8", data)
	})

	r.GET("/asset/:id/*path", func(c *gin.Context) {
		idStr := c.Param("id")
		assetPath := c.Param("path")
		
		assetPath = strings.TrimPrefix(assetPath, "/")
		localPath := fmt.Sprintf("data/local_sources/%s/assets/%s", idStr, assetPath)
		
		data, err := os.ReadFile(localPath)
		if err != nil {
			c.JSON(404, gin.H{"error": "资源不存在"})
			return
		}
		
		contentType := "application/octet-stream"
		if strings.HasSuffix(assetPath, ".js") {
			contentType = "application/javascript"
		} else if strings.HasSuffix(assetPath, ".json") {
			contentType = "application/json"
		} else if strings.HasSuffix(assetPath, ".jar") {
			contentType = "application/java-archive"
		}
		
		c.Data(200, contentType, data)
	})

	api.RegisterRoutes(r)

	println("FluxBox 启动成功，访问地址: http://localhost:20504")
	r.Run(":20504")
}

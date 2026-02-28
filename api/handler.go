package api

import (
	"FluxBox/models"
	"FluxBox/parser"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	SourcesFile   = "data/sources.json"
	CacheFile     = "data/config_cache.json"
	MemorySources []models.SourceItem
	MemoryConfig  models.TVConfig
	Mu            sync.Mutex
	IsAggregating bool
)

// InitData 初始化数据：从文件读取到内存
func InitData() {
	// 加载源列表
	if data, err := os.ReadFile(SourcesFile); err == nil {
		json.Unmarshal(data, &MemorySources)
	} else {
		MemorySources = []models.SourceItem{}
	}

	// 加载缓存配置
	if data, err := os.ReadFile(CacheFile); err == nil {
		json.Unmarshal(data, &MemoryConfig)
	}
}

// saveSourcesToFile 保存源列表到文件
func saveSourcesToFile() {
	data, _ := json.MarshalIndent(MemorySources, "", "  ")
	os.WriteFile(SourcesFile, data, 0644)
}

// fetchAndParse 抓取并脱壳的核心函数
func fetchAndParse(url string) (*models.TVConfig, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "okhttp/3.15.0")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if (len(body) > 2 && body[0] == 0x1f && body[1] == 0x8b) || resp.Header.Get("Content-Encoding") == "gzip" {
		reader, _ := gzip.NewReader(bytes.NewReader(body))
		if reader != nil {
			body, _ = io.ReadAll(reader)
			reader.Close()
		}
	}

	jsonStr, err := parser.ParseConfig(string(body))
	if err != nil {
		return nil, err
	}

	var config models.TVConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("JSON 解析失败")
	}
	return &config, nil
}

// RunAggregation 执行聚合任务 (带 Key 前缀和 Jar 强制注入)
func RunAggregation() {
	Mu.Lock()
	if IsAggregating {
		Mu.Unlock()
		return
	}
	IsAggregating = true
	Mu.Unlock()

	defer func() {
		Mu.Lock()
		IsAggregating = false
		Mu.Unlock()
	}()

	final := models.TVConfig{
		Wallpaper: "https://pic1.aj7.cloud/2024/02/14/65cc5c76c024d.jpg",
		Sites:     []models.Site{},
	}

	var wg sync.WaitGroup
	// 临时存放所有站点的切片
	var allSites []models.Site
	var sitesMu sync.Mutex

	for i := range MemorySources {
		if !MemorySources[i].Enabled {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			src := &MemorySources[idx]
			config, err := fetchAndParse(src.URL)

			Mu.Lock()
			if err != nil {
				src.LastStatus = "failed"
				src.LastError = err.Error()
				Mu.Unlock()
			} else {
				src.LastStatus = "success"
				src.LastError = ""

				// 确定当前源的 Jar 地址 (有些源叫 spider)
				currentJar := config.Spider

				// 核心注入逻辑
				for _, s := range config.Sites {
					// 1. 修改 Key 防止冲突
					s.Key = fmt.Sprintf("fb_%d_%s", src.ID, s.Key)

					// 2. 注入私有 Jar (这是解决多仓/Jar不匹配的关键)
					// 如果站点本身没写 jar，就强行把该源的 spider 塞进去
					if s.Jar == "" && currentJar != "" {
						s.Jar = currentJar
					}

					sitesMu.Lock()
					allSites = append(allSites, s)
					sitesMu.Unlock()
				}

				// 保底设置一个全局 spider (取第一个成功源的)
				if final.Spider == "" {
					final.Spider = currentJar
				}
				Mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	// 站点去重逻辑 (基于注入后的唯一 Key)
	uniqueSites := []models.Site{}
	seen := make(map[string]bool)
	for _, s := range allSites {
		if s.Key != "" && !seen[s.Key] {
			seen[s.Key] = true
			uniqueSites = append(uniqueSites, s)
		}
	}
	final.Sites = uniqueSites

	// 更新内存并落盘
	Mu.Lock()
	MemoryConfig = final
	cacheData, _ := json.MarshalIndent(final, "", "  ")
	os.WriteFile(CacheFile, cacheData, 0644)
	saveSourcesToFile()
	Mu.Unlock()
}

// GetStatus Handler: 获取状态
func GetStatus(c *gin.Context) {
	Mu.Lock()
	defer Mu.Unlock()
	c.JSON(200, gin.H{
		"sources":    MemorySources,
		"is_running": IsAggregating,
		"site_count": len(MemoryConfig.Sites),
	})
}

// AddSource Handler: 添加源
func AddSource(c *gin.Context) {
	var item models.SourceItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(400, gin.H{"error": "无效输入"})
		return
	}
	Mu.Lock()
	item.ID = int(time.Now().UnixNano() / 1e6)
	item.Enabled = true
	MemorySources = append(MemorySources, item)
	saveSourcesToFile()
	Mu.Unlock()
	c.JSON(200, gin.H{"message": "添加成功"})
}

// DeleteSource Handler: 删除源
func DeleteSource(c *gin.Context) {
	idStr := c.Param("id")
	Mu.Lock()
	defer Mu.Unlock()
	for i, s := range MemorySources {
		if fmt.Sprintf("%d", s.ID) == idStr {
			MemorySources = append(MemorySources[:i], MemorySources[i+1:]...)
			saveSourcesToFile()
			c.JSON(200, gin.H{"message": "删除成功"})
			return
		}
	}
	c.JSON(404, gin.H{"error": "未找到该源"})
}

// TriggerAggregate Handler: 触发聚合
func TriggerAggregate(c *gin.Context) {
	go RunAggregation()
	c.JSON(200, gin.H{"message": "任务已启动"})
}

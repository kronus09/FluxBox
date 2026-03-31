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
	SourcesFile       = "data/sources.json"
	CacheFile         = "data/config_cache.json"
	FilterWordsFile   = "data/filter_words.json"
	MemorySources     []models.SourceItem
	MemoryConfig      models.TVConfig
	MemoryFilterWords []string
	Mu                sync.Mutex
	IsAggregating     bool
)

// DefaultFilterWords 默认过滤关键词
var DefaultFilterWords = []string{"直播", "儿童", "启蒙", "教育", "课堂", "学习", "少儿"}

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

	// 加载过滤关键词
	if data, err := os.ReadFile(FilterWordsFile); err == nil && len(data) > 0 {
		json.Unmarshal(data, &MemoryFilterWords)
	} else {
		MemoryFilterWords = append([]string{}, DefaultFilterWords...)
		saveFilterWordsToFile()
	}
}

// saveSourcesToFile 保存源列表到文件
func saveSourcesToFile() {
	data, _ := json.MarshalIndent(MemorySources, "", "  ")
	os.WriteFile(SourcesFile, data, 0644)
}

// saveFilterWordsToFile 保存过滤关键词到文件
func saveFilterWordsToFile() {
	data, _ := json.MarshalIndent(MemoryFilterWords, "", "  ")
	os.WriteFile(FilterWordsFile, data, 0644)
}

// shouldFilterSite 判断站点是否应该被过滤
func shouldFilterSite(name string) bool {
	for _, word := range MemoryFilterWords {
		if len(word) > 0 && len(name) > 0 {
			for i := 0; i <= len(name)-len(word); i++ {
				if name[i:i+len(word)] == word {
					return true
				}
			}
		}
	}
	return false
}

// fetchAndParse 抓取并脱壳的核心函数，返回配置和响应时间(毫秒)
func fetchAndParse(url string) (*models.TVConfig, int, error) {
	start := time.Now()
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "okhttp/3.15.0")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
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
		return nil, int(time.Since(start).Milliseconds()), err
	}

	var config models.TVConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, int(time.Since(start).Milliseconds()), fmt.Errorf("JSON 解析失败")
	}
	return &config, int(time.Since(start).Milliseconds()), nil
}

// testSiteSpeed 测试单个站点的响应速度，返回响应时间(毫秒)
func testSiteSpeed(apiUrl string) int {
	if apiUrl == "" {
		return 99999
	}
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", apiUrl, nil)
	req.Header.Set("User-Agent", "okhttp/3.15.0")

	resp, err := client.Do(req)
	if err != nil {
		return 99999
	}
	defer resp.Body.Close()
	return int(time.Since(start).Milliseconds())
}

// RunAggregation 执行聚合任务 (带 Key 前缀和 Jar 强制注入)
// mode: "all" 全部站点, "fastest" 只取前120个站点
// 对每个站点单独测速并按响应时间排序
func RunAggregation(mode string) {
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

	type sourceResult struct {
		idx    int
		config *models.TVConfig
	}

	var wg sync.WaitGroup
	results := make([]sourceResult, 0)
	var resultsMu sync.Mutex

	for i := range MemorySources {
		if !MemorySources[i].Enabled {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			src := &MemorySources[idx]
			config, responseTime, err := fetchAndParse(src.URL)

			Mu.Lock()
			if err != nil {
				src.LastStatus = "failed"
				src.LastError = err.Error()
				src.ResponseTime = responseTime
				Mu.Unlock()
			} else {
				src.LastStatus = "success"
				src.LastError = ""
				src.ResponseTime = responseTime
				Mu.Unlock()
				resultsMu.Lock()
				results = append(results, sourceResult{idx: idx, config: config})
				resultsMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	var allSites []models.Site
	for _, r := range results {
		src := &MemorySources[r.idx]
		currentJar := r.config.Spider

		for _, s := range r.config.Sites {
			if shouldFilterSite(s.Name) {
				continue
			}
			s.Key = fmt.Sprintf("fb_%d_%s", src.ID, s.Key)
			if s.Jar == "" && currentJar != "" {
				s.Jar = currentJar
			}
			allSites = append(allSites, s)
		}

		if final.Spider == "" {
			final.Spider = currentJar
		}
	}

	uniqueSites := []models.Site{}
	seen := make(map[string]bool)
	for _, s := range allSites {
		if s.Key != "" && !seen[s.Key] {
			seen[s.Key] = true
			uniqueSites = append(uniqueSites, s)
		}
	}

	var speedWg sync.WaitGroup
	for i := range uniqueSites {
		speedWg.Add(1)
		go func(idx int) {
			defer speedWg.Done()
			uniqueSites[idx].Speed = testSiteSpeed(uniqueSites[idx].Api)
		}(i)
	}
	speedWg.Wait()

	for i := 0; i < len(uniqueSites); i++ {
		for j := i + 1; j < len(uniqueSites); j++ {
			if uniqueSites[j].Speed < uniqueSites[i].Speed {
				uniqueSites[i], uniqueSites[j] = uniqueSites[j], uniqueSites[i]
			}
		}
	}

	if mode == "fastest" && len(uniqueSites) > 120 {
		uniqueSites = uniqueSites[:120]
	}

	final.Sites = uniqueSites

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
	var req struct {
		Mode string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Mode == "" {
		req.Mode = "fastest"
	}
	go RunAggregation(req.Mode)
	c.JSON(200, gin.H{"message": "任务已启动"})
}

// UpdateSource Handler: 更新源
func UpdateSource(c *gin.Context) {
	var item models.SourceItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(400, gin.H{"error": "无效输入"})
		return
	}
	Mu.Lock()
	defer Mu.Unlock()
	for i, s := range MemorySources {
		if s.ID == item.ID {
			MemorySources[i].Name = item.Name
			MemorySources[i].URL = item.URL
			saveSourcesToFile()
			c.JSON(200, gin.H{"message": "更新成功"})
			return
		}
	}
	c.JSON(404, gin.H{"error": "未找到该源"})
}

// TestSource Handler: 测试单个源
func TestSource(c *gin.Context) {
	idStr := c.Param("id")
	Mu.Lock()
	var targetSource *models.SourceItem
	for i := range MemorySources {
		if fmt.Sprintf("%d", MemorySources[i].ID) == idStr {
			targetSource = &MemorySources[i]
			break
		}
	}
	Mu.Unlock()

	if targetSource == nil {
		c.JSON(404, gin.H{"error": "未找到该源", "success": false})
		return
	}

	_, responseTime, err := fetchAndParse(targetSource.URL)

	Mu.Lock()
	if err != nil {
		targetSource.LastStatus = "failed"
		targetSource.LastError = err.Error()
		targetSource.ResponseTime = responseTime
		Mu.Unlock()
		c.JSON(200, gin.H{
			"success":      false,
			"error":        err.Error(),
			"id":           targetSource.ID,
			"responseTime": responseTime,
		})
	} else {
		targetSource.LastStatus = "success"
		targetSource.LastError = ""
		targetSource.ResponseTime = responseTime
		Mu.Unlock()
		c.JSON(200, gin.H{
			"success":      true,
			"id":           targetSource.ID,
			"responseTime": responseTime,
		})
	}
}

// TestSources Handler: 批量测试多个源
func TestSources(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效输入"})
		return
	}

	type TestResult struct {
		ID           int    `json:"id"`
		Success      bool   `json:"success"`
		Error        string `json:"error,omitempty"`
		ResponseTime int    `json:"responseTime"`
	}

	results := make([]TestResult, 0)
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	for _, id := range req.IDs {
		wg.Add(1)
		go func(sourceID int) {
			defer wg.Done()

			Mu.Lock()
			var targetSource *models.SourceItem
			for i := range MemorySources {
				if MemorySources[i].ID == sourceID {
					targetSource = &MemorySources[i]
					break
				}
			}
			Mu.Unlock()

			if targetSource == nil {
				resultsMu.Lock()
				results = append(results, TestResult{ID: sourceID, Success: false, Error: "未找到该源"})
				resultsMu.Unlock()
				return
			}

			_, responseTime, err := fetchAndParse(targetSource.URL)

			Mu.Lock()
			if err != nil {
				targetSource.LastStatus = "failed"
				targetSource.LastError = err.Error()
				targetSource.ResponseTime = responseTime
				Mu.Unlock()
				resultsMu.Lock()
				results = append(results, TestResult{ID: sourceID, Success: false, Error: err.Error(), ResponseTime: responseTime})
				resultsMu.Unlock()
			} else {
				targetSource.LastStatus = "success"
				targetSource.LastError = ""
				targetSource.ResponseTime = responseTime
				Mu.Unlock()
				resultsMu.Lock()
				results = append(results, TestResult{ID: sourceID, Success: true, ResponseTime: responseTime})
				resultsMu.Unlock()
			}
		}(id)
	}
	wg.Wait()

	c.JSON(200, gin.H{"results": results})
}

// GetFilterWords Handler: 获取过滤关键词列表
func GetFilterWords(c *gin.Context) {
	Mu.Lock()
	defer Mu.Unlock()
	c.JSON(200, gin.H{"words": MemoryFilterWords})
}

// AddFilterWord Handler: 添加过滤关键词（支持单个或批量）
func AddFilterWord(c *gin.Context) {
	var req struct {
		Word  string   `json:"word"`
		Words []string `json:"words"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效输入"})
		return
	}

	Mu.Lock()
	defer Mu.Unlock()

	if len(req.Words) > 0 {
		MemoryFilterWords = req.Words
		saveFilterWordsToFile()
		c.JSON(200, gin.H{"message": "保存成功"})
		return
	}

	if req.Word == "" {
		c.JSON(400, gin.H{"error": "无效输入"})
		return
	}

	for _, w := range MemoryFilterWords {
		if w == req.Word {
			c.JSON(200, gin.H{"message": "关键词已存在"})
			return
		}
	}

	MemoryFilterWords = append(MemoryFilterWords, req.Word)
	saveFilterWordsToFile()
	c.JSON(200, gin.H{"message": "添加成功"})
}

// DeleteFilterWord Handler: 删除过滤关键词
func DeleteFilterWord(c *gin.Context) {
	word := c.Param("word")

	Mu.Lock()
	defer Mu.Unlock()

	for i, w := range MemoryFilterWords {
		if w == word {
			MemoryFilterWords = append(MemoryFilterWords[:i], MemoryFilterWords[i+1:]...)
			saveFilterWordsToFile()
			c.JSON(200, gin.H{"message": "删除成功"})
			return
		}
	}
	c.JSON(404, gin.H{"error": "未找到该关键词"})
}

// ResetFilterWords Handler: 重置过滤关键词为默认值
func ResetFilterWords(c *gin.Context) {
	Mu.Lock()
	defer Mu.Unlock()

	MemoryFilterWords = append([]string{}, DefaultFilterWords...)
	saveFilterWordsToFile()
	c.JSON(200, gin.H{"message": "重置成功"})
}

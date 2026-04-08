package api

import (
	"FluxBox/models"
	"FluxBox/parser"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	SourcesFile         = "data/sources.json"
	CacheFile           = "data/config_cache.json"
	FilterWordsFile     = "data/filter_words.json"
	ScheduleFile        = "data/schedule.json"
	MultiConfigFile     = "data/multi_config.json"
	SourceCacheDir      = "data/sources/"
	HomeMenuSourceFile  = "data/home_menu_source.json"
	GlobalConfigFile    = "data/global_config.json"
	MemorySources       []models.SourceItem
	MemoryConfig        models.TVConfig
	MemoryFilterWords   []string
	MemorySchedule      models.ScheduleConfig
	MemoryMultiConfig   models.TVConfig
	MemoryHomeMenuSource int
	MemoryGlobalConfig  models.GlobalConfig
	Mu                  sync.Mutex
	IsAggregating       bool
	SchedulerTimer      *time.Timer
	HealthSchedulerTimer *time.Timer
)

// DefaultFilterWords 默认过滤关键词
var DefaultFilterWords = []string{"直播", "儿童", "启蒙", "教育", "课堂", "学习", "少儿", "预告"}

// InitData 初始化数据：从文件读取到内存
func InitData() {
	os.MkdirAll("data", 0755)
	os.MkdirAll(SourceCacheDir, 0755)

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

	// 加载计划任务配置
	if data, err := os.ReadFile(ScheduleFile); err == nil && len(data) > 0 {
		json.Unmarshal(data, &MemorySchedule)
		if MemorySchedule.MaxSites == 0 {
			MemorySchedule.MaxSites = 120
		}
	} else {
		MemorySchedule = models.ScheduleConfig{
			Enabled:   false,
			Frequency: "daily",
			Time:      "04:00",
			Days:      []int{1, 2, 3, 4, 5},
			Mode:      "fastest",
			MaxSites:  120,
		}
		saveScheduleToFile()
	}

	// 加载多仓配置
	if data, err := os.ReadFile(MultiConfigFile); err == nil {
		json.Unmarshal(data, &MemoryMultiConfig)
	}

	// 加载首页菜单源配置
	if data, err := os.ReadFile(HomeMenuSourceFile); err == nil && len(data) > 0 {
		json.Unmarshal(data, &MemoryHomeMenuSource)
	}

	// 加载全局配置
	if data, err := os.ReadFile(GlobalConfigFile); err == nil && len(data) > 0 {
		json.Unmarshal(data, &MemoryGlobalConfig)
		// 同步 filter_words 到 MemoryFilterWords
		if len(MemoryGlobalConfig.FilterWords) > 0 {
			MemoryFilterWords = MemoryGlobalConfig.FilterWords
			saveFilterWordsToFile()
		}
	} else {
		MemoryGlobalConfig = models.GlobalConfig{
			AggMode:              "fastest",
			MaxSites:             120,
			FilterWords:          append([]string{}, DefaultFilterWords...),
			HomeMenuSource:       0,
			MultiIncludeWarning:  false,
			MultiPreferLocal:     true,
			AutoDisableUnhealthy: true,
			AutoDisableWarning:   false,
			AutoDisableFailed:    true,
			ScheduleEnabled:      false,
			AggSingleEnabled:     true,
			AggMultiEnabled:      true,
			AggScheduleFreq:      "daily",
			AggScheduleTime:      "05:00",
			AggScheduleDays:      []int{1, 2, 3, 4, 5},
			HealthScheduleEnabled: false,
			HealthScheduleFreq:    "daily",
			HealthScheduleTime:    "04:00",
			HealthScheduleDays:    []int{1, 2, 3, 4, 5},
		}
		MemoryFilterWords = MemoryGlobalConfig.FilterWords
		saveGlobalConfigToFile()
		saveFilterWordsToFile()
	}

	// 启动计划任务调度器
	StartScheduler()
}

// saveSourcesToFile 保存源列表到文件
func saveSourcesToFile() error {
	data, err := json.MarshalIndent(MemorySources, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SourcesFile, data, 0644)
}

// saveFilterWordsToFile 保存过滤关键词到文件
func saveFilterWordsToFile() {
	data, _ := json.MarshalIndent(MemoryFilterWords, "", "  ")
	os.WriteFile(FilterWordsFile, data, 0644)
}

// saveScheduleToFile 保存计划任务配置到文件
func saveScheduleToFile() {
	data, _ := json.MarshalIndent(MemorySchedule, "", "  ")
	os.WriteFile(ScheduleFile, data, 0644)
}

// saveGlobalConfigToFile 保存全局配置到文件
func saveGlobalConfigToFile() error {
	data, err := json.MarshalIndent(MemoryGlobalConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GlobalConfigFile, data, 0644)
}

// saveMultiConfigToFile 保存多仓配置到文件
func saveMultiConfigToFile() {
	data, _ := json.MarshalIndent(MemoryMultiConfig, "", "  ")
	os.WriteFile(MultiConfigFile, data, 0644)
}

// saveHomeMenuSourceToFile 保存首页菜单源配置到文件
func saveHomeMenuSourceToFile() {
	data, _ := json.MarshalIndent(MemoryHomeMenuSource, "", "  ")
	os.WriteFile(HomeMenuSourceFile, data, 0644)
}

// StartScheduler 启动计划任务调度器
func StartScheduler() {
	Mu.Lock()
	defer Mu.Unlock()

	// 停止现有的定时器
	if SchedulerTimer != nil {
		SchedulerTimer.Stop()
		SchedulerTimer = nil
	}
	if HealthSchedulerTimer != nil {
		HealthSchedulerTimer.Stop()
		HealthSchedulerTimer = nil
	}

	// 如果计划任务总开关未开启，直接返回
	if !MemoryGlobalConfig.ScheduleEnabled {
		return
	}

	// 启动聚合任务调度器
	if MemoryGlobalConfig.AggSingleEnabled || MemoryGlobalConfig.AggMultiEnabled {
		nextTime := calculateNextAggRunTime()
		if !nextTime.IsZero() {
			duration := time.Until(nextTime)
			SchedulerTimer = time.AfterFunc(duration, func() {
				Mu.Lock()
				config := MemoryGlobalConfig
				Mu.Unlock()

				if config.ScheduleEnabled {
					// 执行单仓聚合
					if config.AggSingleEnabled {
						RunAggregation(config.AggMode)
					}
					// 执行多仓生成
					if config.AggMultiEnabled {
						GenerateMultiConfigInternal()
					}
				}
				StartScheduler()
			})
		}
	}

	// 启动健康检查任务调度器
	if MemoryGlobalConfig.HealthScheduleEnabled {
		nextTime := calculateNextHealthRunTime()
		if !nextTime.IsZero() {
			duration := time.Until(nextTime)
			HealthSchedulerTimer = time.AfterFunc(duration, func() {
				Mu.Lock()
				config := MemoryGlobalConfig
				Mu.Unlock()

				if config.HealthScheduleEnabled {
					CheckAllHealthInternal()
				}
				StartScheduler()
			})
		}
	}
}

// calculateNextAggRunTime 计算聚合任务下次执行时间
func calculateNextAggRunTime() time.Time {
	now := time.Now()

	hour, minute := 0, 0
	fmt.Sscanf(MemoryGlobalConfig.AggScheduleTime, "%d:%d", &hour, &minute)

	if MemoryGlobalConfig.AggScheduleFreq == "daily" {
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if next.Before(now) || next.Equal(now) {
			next = next.Add(24 * time.Hour)
		}
		return next
	}

	if MemoryGlobalConfig.AggScheduleFreq == "weekly" {
		currentWeekday := int(now.Weekday())
		if currentWeekday == 0 {
			currentWeekday = 7
		}

		var nextDay int
		found := false
		for _, d := range MemoryGlobalConfig.AggScheduleDays {
			if d > currentWeekday {
				nextDay = d
				found = true
				break
			}
		}

		if !found && len(MemoryGlobalConfig.AggScheduleDays) > 0 {
			nextDay = MemoryGlobalConfig.AggScheduleDays[0]
		} else if !found {
			return time.Time{}
		}

		daysToAdd := nextDay - currentWeekday
		if daysToAdd <= 0 {
			daysToAdd += 7
		}

		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		next = next.Add(time.Duration(daysToAdd) * 24 * time.Hour)
		if next.Before(now) || next.Equal(now) {
			next = next.Add(7 * 24 * time.Hour)
		}
		return next
	}

	return time.Time{}
}

// calculateNextHealthRunTime 计算健康检查任务下次执行时间
func calculateNextHealthRunTime() time.Time {
	now := time.Now()

	hour, minute := 0, 0
	fmt.Sscanf(MemoryGlobalConfig.HealthScheduleTime, "%d:%d", &hour, &minute)

	if MemoryGlobalConfig.HealthScheduleFreq == "daily" {
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if next.Before(now) || next.Equal(now) {
			next = next.Add(24 * time.Hour)
		}
		return next
	}

	if MemoryGlobalConfig.HealthScheduleFreq == "weekly" {
		currentWeekday := int(now.Weekday())
		if currentWeekday == 0 {
			currentWeekday = 7
		}

		var nextDay int
		found := false
		for _, d := range MemoryGlobalConfig.HealthScheduleDays {
			if d > currentWeekday {
				nextDay = d
				found = true
				break
			}
		}

		if !found && len(MemoryGlobalConfig.HealthScheduleDays) > 0 {
			nextDay = MemoryGlobalConfig.HealthScheduleDays[0]
		} else if !found {
			return time.Time{}
		}

		daysToAdd := nextDay - currentWeekday
		if daysToAdd <= 0 {
			daysToAdd += 7
		}

		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		next = next.Add(time.Duration(daysToAdd) * 24 * time.Hour)
		if next.Before(now) || next.Equal(now) {
			next = next.Add(7 * 24 * time.Hour)
		}
		return next
	}

	return time.Time{}
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
// 支持单仓和多仓格式
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
		return nil, int(time.Since(start).Milliseconds()), fmt.Errorf("JSON 解析失败: %v", err)
	}

	// 检测是否为多仓格式
	if len(config.VideoList) > 0 && len(config.Sites) == 0 {
		return fetchMultiWarehouse(&config, start)
	}

	return &config, int(time.Since(start).Milliseconds()), nil
}

// fetchMultiWarehouse 处理多仓格式，递归获取所有子仓库配置
func fetchMultiWarehouse(config *models.TVConfig, startTime time.Time) (*models.TVConfig, int, error) {
	type warehouseResult struct {
		config *models.TVConfig
		err    error
	}

	results := make(chan warehouseResult, len(config.VideoList))
	var wg sync.WaitGroup

	for _, vw := range config.VideoList {
		if vw.URL == "" {
			continue
		}
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			subConfig, _, err := fetchAndParse(url)
			results <- warehouseResult{config: subConfig, err: err}
		}(vw.URL)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	merged := &models.TVConfig{
		Wallpaper: config.Wallpaper,
		Logo:      config.Logo,
		Spider:    config.Spider,
		Sites:     []models.Site{},
		Lives:     []models.Live{},
		Ads:       []string{},
		Parses:    []models.Parse{},
		Rules:     []models.Rule{},
		Flags:     []string{},
		Ijk:       []models.Ijk{},
		Doh:       []models.Doh{},
	}

	for result := range results {
		if result.err != nil || result.config == nil {
			continue
		}
		sub := result.config

		// 合并站点
		if len(sub.Sites) > 0 {
			merged.Sites = append(merged.Sites, sub.Sites...)
		}

		// 合并直播源
		if len(sub.Lives) > 0 {
			merged.Lives = append(merged.Lives, sub.Lives...)
		}

		// 合并广告过滤
		if len(sub.Ads) > 0 {
			merged.Ads = append(merged.Ads, sub.Ads...)
		}

		// 合并解析器
		if len(sub.Parses) > 0 {
			merged.Parses = append(merged.Parses, sub.Parses...)
		}

		// 合并规则
		if len(sub.Rules) > 0 {
			merged.Rules = append(merged.Rules, sub.Rules...)
		}

		// 合并 Flags
		if len(sub.Flags) > 0 {
			merged.Flags = append(merged.Flags, sub.Flags...)
		}

		// 合并 IJK 配置
		if len(sub.Ijk) > 0 {
			merged.Ijk = append(merged.Ijk, sub.Ijk...)
		}

		// 合并 DoH 配置
		if len(sub.Doh) > 0 {
			merged.Doh = append(merged.Doh, sub.Doh...)
		}

		// 继承第一个有效的 Spider
		if merged.Spider == "" && sub.Spider != "" {
			merged.Spider = sub.Spider
		}

		// 继承第一个有效的 Wallpaper
		if merged.Wallpaper == "" && sub.Wallpaper != "" {
			merged.Wallpaper = sub.Wallpaper
		}
	}

	return merged, int(time.Since(startTime).Milliseconds()), nil
}

func cleanExt(ext interface{}) interface{} {
	if ext == nil {
		return nil
	}
	
	if extMap, ok := ext.(map[string]interface{}); ok {
		cleaned := make(map[string]interface{})
		popupFields := []string{"msg", "logo", "message", "popup", "弹窗", "提示", "公告"}
		
		for key, value := range extMap {
			isPopup := false
			for _, field := range popupFields {
				if key == field {
					isPopup = true
					break
				}
			}
			if !isPopup {
				cleaned[key] = value
			}
		}
		
		if len(cleaned) == 0 {
			return nil
		}
		return cleaned
	}
	
	return ext
}

func getSourceCachePath(sourceID int) string {
	return fmt.Sprintf("%s%d.json", SourceCacheDir, sourceID)
}

func saveSourceCache(sourceID int, config *models.TVConfig) error {
	cachePath := getSourceCachePath(sourceID)
	cacheData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("缓存源配置失败 (序列化): sourceID=%d, error=%v", sourceID, err)
		return err
	}
	err = os.WriteFile(cachePath, cacheData, 0644)
	if err != nil {
		log.Printf("缓存源配置失败 (写入): sourceID=%d, path=%s, error=%v", sourceID, cachePath, err)
		return err
	}
	log.Printf("缓存源配置成功: sourceID=%d, path=%s", sourceID, cachePath)
	return nil
}

func deleteSourceCache(sourceID int) error {
	cachePath := getSourceCachePath(sourceID)
	return os.Remove(cachePath)
}

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
		sourceID int
		config   *models.TVConfig
		source   *models.SourceItem
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
			
			Mu.Lock()
			src := MemorySources[idx]
			Mu.Unlock()
			
			if NeedHealthCheck(&src) {
				result := CheckSourceHealth(&src)
				Mu.Lock()
				for i := range MemorySources {
					if MemorySources[i].ID == src.ID {
						UpdateSourceHealth(&MemorySources[i], result)
						break
					}
				}
				Mu.Unlock()
			}
			
			config, responseTime, err := fetchAndParse(src.URL)

			Mu.Lock()
			for i := range MemorySources {
				if MemorySources[i].ID == src.ID {
					if err != nil {
						MemorySources[i].LastStatus = "failed"
						MemorySources[i].LastError = err.Error()
						MemorySources[i].ResponseTime = responseTime
					} else {
						MemorySources[i].LastStatus = "success"
						MemorySources[i].LastError = ""
						MemorySources[i].ResponseTime = responseTime
					}
					break
				}
			}
			Mu.Unlock()
			
			if err == nil {
				resultsMu.Lock()
				results = append(results, sourceResult{sourceID: src.ID, config: config})
				resultsMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	Mu.Lock()
	saveSourcesToFile()
	Mu.Unlock()

	var allSites []models.Site
	var homeMenuSites []models.Site
	globalConfigSet := false
	var otherSites []models.Site
	
	Mu.Lock()
	homeMenuSourceID := MemoryHomeMenuSource
	Mu.Unlock()

	var fastestSourceID int
	if homeMenuSourceID == 0 {
		fastestTime := -1
		for _, src := range MemorySources {
			if !src.Enabled || src.LastStatus != "success" {
				continue
			}
			if fastestTime == -1 || src.ResponseTime < fastestTime {
				fastestTime = src.ResponseTime
				fastestSourceID = src.ID
			}
		}
	}
	
	for _, r := range results {
		Mu.Lock()
		var srcURL string
		for i := range MemorySources {
			if MemorySources[i].ID == r.sourceID {
				srcURL = MemorySources[i].URL
				break
			}
		}
		Mu.Unlock()
		
		replaced, skipped := CacheSourceJars(r.sourceID, srcURL, r.config)
		if skipped > 0 {
			log.Printf("聚合源有jar缺失: sourceID=%d replaced=%d skipped=%d", r.sourceID, replaced, skipped)
		}
		saveSourceCache(r.sourceID, r.config)
		
		Mu.Lock()
		var src *models.SourceItem
		for i := range MemorySources {
			if MemorySources[i].ID == r.sourceID {
				src = &MemorySources[i]
				break
			}
		}
		Mu.Unlock()
		
		if src == nil {
			continue
		}
		
		currentJar := r.config.Spider
		isHomeMenuSource := (homeMenuSourceID != 0 && r.sourceID == homeMenuSourceID) || 
			(homeMenuSourceID == 0 && r.sourceID == fastestSourceID)

		firstSite := true
		for _, s := range r.config.Sites {
			if shouldFilterSite(s.Name) {
				continue
			}
			
			if !ShouldIncludeSite(&s, src, r.config) {
				continue
			}
			
			s.Key = fmt.Sprintf("fb_%d_%s", src.ID, s.Key)
			if s.Jar == "" && currentJar != "" && s.Type == 3 {
				s.Jar = currentJar
			}
			s.Ext = cleanExt(s.Ext)
			
			if isHomeMenuSource {
				if firstSite {
					s.Name = "FluxBox聚合源"
					firstSite = false
				}
				homeMenuSites = append(homeMenuSites, s)
			} else {
				if s.Searchable == 1 {
					otherSites = append(otherSites, s)
				}
			}
		}

		if final.Spider == "" {
			final.Spider = currentJar
		}
		
		if !globalConfigSet || isHomeMenuSource {
			if len(r.config.Lives) > 0 {
				final.Lives = r.config.Lives
			}
			if len(r.config.Parses) > 0 {
				final.Parses = r.config.Parses
			}
			if len(r.config.Rules) > 0 {
				final.Rules = r.config.Rules
			}
			if len(r.config.Flags) > 0 {
				final.Flags = r.config.Flags
			}
			if len(r.config.Doh) > 0 {
				final.Doh = r.config.Doh
			}
			if len(r.config.Ads) > 0 {
				final.Ads = r.config.Ads
			}
			if len(r.config.Ijk) > 0 {
				final.Ijk = r.config.Ijk
			}
			if isHomeMenuSource {
				globalConfigSet = true
			} else if !globalConfigSet {
				globalConfigSet = true
			}
		}
	}
	
	allSites = append(homeMenuSites, otherSites...)

	uniqueSites := []models.Site{}
	seen := make(map[string]bool)
	for _, s := range allSites {
		if s.Key != "" && !seen[s.Key] {
			seen[s.Key] = true
			uniqueSites = append(uniqueSites, s)
		}
	}

	homeSiteCount := len(homeMenuSites)
	var homeUniqueSites []models.Site
	var otherUniqueSites []models.Site
	
	if homeSiteCount > 0 {
		for _, s := range uniqueSites {
			isHomeSite := false
			for _, hs := range homeMenuSites {
				if s.Key == hs.Key {
					isHomeSite = true
					break
				}
			}
			if isHomeSite {
				homeUniqueSites = append(homeUniqueSites, s)
			} else {
				otherUniqueSites = append(otherUniqueSites, s)
			}
		}
	} else {
		otherUniqueSites = uniqueSites
	}

	var speedWg sync.WaitGroup
	for i := range otherUniqueSites {
		speedWg.Add(1)
		go func(idx int) {
			defer speedWg.Done()
			otherUniqueSites[idx].Speed = testSiteSpeed(otherUniqueSites[idx].Api)
		}(i)
	}
	speedWg.Wait()

	for i := 0; i < len(otherUniqueSites); i++ {
		for j := i + 1; j < len(otherUniqueSites); j++ {
			if otherUniqueSites[j].Speed < otherUniqueSites[i].Speed {
				otherUniqueSites[i], otherUniqueSites[j] = otherUniqueSites[j], otherUniqueSites[i]
			}
		}
	}

	if mode == "fastest" {
		Mu.Lock()
		maxTotal := MemorySchedule.MaxSites
		if maxTotal == 0 {
			maxTotal = 120
		}
		Mu.Unlock()
		remaining := maxTotal - len(homeUniqueSites)
		if remaining > 0 && len(otherUniqueSites) > remaining {
			otherUniqueSites = otherUniqueSites[:remaining]
		} else if remaining <= 0 {
			otherUniqueSites = nil
		}
	}

	final.Sites = append(homeUniqueSites, otherUniqueSites...)

	Mu.Lock()
	MemoryConfig = final
	cacheData, _ := json.MarshalIndent(final, "", "  ")
	os.WriteFile(CacheFile, cacheData, 0644)
	if err := saveSourcesToFile(); err != nil {
		log.Printf("保存源列表失败: %v", err)
	}
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
	if err := saveSourcesToFile(); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		Mu.Unlock()
		return
	}
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
			sourceID := s.ID
			MemorySources = append(MemorySources[:i], MemorySources[i+1:]...)
			if err := saveSourcesToFile(); err != nil {
				c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
				return
			}
			deleteSourceCache(sourceID)
			ClearSourceJarCache(sourceID)
			DeleteLocalSource(sourceID)
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
			if err := saveSourcesToFile(); err != nil {
				c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
				return
			}
			c.JSON(200, gin.H{"message": "更新成功"})
			return
		}
	}
	c.JSON(404, gin.H{"error": "未找到该源"})
}

// ToggleSource Handler: 切换源启用状态
func ToggleSource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效ID"})
		return
	}
	Mu.Lock()
	defer Mu.Unlock()
	for i, s := range MemorySources {
		if s.ID == id {
			MemorySources[i].Enabled = !s.Enabled
			if err := saveSourcesToFile(); err != nil {
				c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
				return
			}
			status := "启用"
			if !MemorySources[i].Enabled {
				status = "禁用"
			}
			c.JSON(200, gin.H{"message": "切换成功", "enabled": MemorySources[i].Enabled, "status": status})
			return
		}
	}
	c.JSON(404, gin.H{"error": "未找到该源"})
}

// TestSource Handler: 测试单个源
func TestSource(c *gin.Context) {
	idStr := c.Param("id")
	Mu.Lock()
	var sourceURL string
	var sourceID int
	found := false
	for i := range MemorySources {
		if fmt.Sprintf("%d", MemorySources[i].ID) == idStr {
			sourceURL = MemorySources[i].URL
			sourceID = MemorySources[i].ID
			found = true
			break
		}
	}
	Mu.Unlock()

	if !found {
		c.JSON(404, gin.H{"error": "未找到该源", "success": false})
		return
	}

	_, responseTime, err := fetchAndParse(sourceURL)

	Mu.Lock()
	for i := range MemorySources {
		if MemorySources[i].ID == sourceID {
			if err != nil {
				MemorySources[i].LastStatus = "failed"
				MemorySources[i].LastError = err.Error()
				MemorySources[i].ResponseTime = responseTime
			} else {
				MemorySources[i].LastStatus = "success"
				MemorySources[i].LastError = ""
				MemorySources[i].ResponseTime = responseTime
			}
			break
		}
	}
	if err := saveSourcesToFile(); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error(), "success": false})
		Mu.Unlock()
		return
	}
	Mu.Unlock()

	if err != nil {
		c.JSON(200, gin.H{
			"success":      false,
			"error":        err.Error(),
			"id":           sourceID,
			"responseTime": responseTime,
		})
	} else {
		c.JSON(200, gin.H{
			"success":      true,
			"id":           sourceID,
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
			var sourceURL string
			found := false
			for i := range MemorySources {
				if MemorySources[i].ID == sourceID {
					sourceURL = MemorySources[i].URL
					found = true
					break
				}
			}
			Mu.Unlock()

			if !found {
				resultsMu.Lock()
				results = append(results, TestResult{ID: sourceID, Success: false, Error: "未找到该源"})
				resultsMu.Unlock()
				return
			}

			_, responseTime, err := fetchAndParse(sourceURL)

			Mu.Lock()
			for i := range MemorySources {
				if MemorySources[i].ID == sourceID {
					if err != nil {
						MemorySources[i].LastStatus = "failed"
						MemorySources[i].LastError = err.Error()
						MemorySources[i].ResponseTime = responseTime
					} else {
						MemorySources[i].LastStatus = "success"
						MemorySources[i].LastError = ""
						MemorySources[i].ResponseTime = responseTime
					}
					break
				}
			}
			Mu.Unlock()

			resultsMu.Lock()
			if err != nil {
				results = append(results, TestResult{ID: sourceID, Success: false, Error: err.Error(), ResponseTime: responseTime})
			} else {
				results = append(results, TestResult{ID: sourceID, Success: true, ResponseTime: responseTime})
			}
			resultsMu.Unlock()
		}(id)
	}
	wg.Wait()

	Mu.Lock()
	if err := saveSourcesToFile(); err != nil {
		log.Printf("保存源列表失败: %v", err)
	}
	Mu.Unlock()

	c.JSON(200, gin.H{"results": results})
}

// CheckSingleHealth Handler: 检查单个源的健康状态
func CheckSingleHealth(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效ID"})
		return
	}

	Mu.Lock()
	var src *models.SourceItem
	for i := range MemorySources {
		if MemorySources[i].ID == id {
			src = &MemorySources[i]
			break
		}
	}
	Mu.Unlock()

	if src == nil {
		c.JSON(404, gin.H{"error": "未找到该源"})
		return
	}

	result := CheckSourceHealth(src)

	Mu.Lock()
	for i := range MemorySources {
		if MemorySources[i].ID == id {
			UpdateSourceHealth(&MemorySources[i], result)
			break
		}
	}
	saveSourcesToFile()
	Mu.Unlock()

	c.JSON(200, gin.H{
		"success":      true,
		"healthScore":  result.HealthScore,
		"healthStatus": result.HealthStatus,
		"siteTotal":    result.SiteTotal,
		"siteCrawler":  result.SiteCrawler,
		"siteCollector": result.SiteCollector,
		"jarTotal":     result.JarTotal,
		"jarSuccess":   result.JarSuccess,
		"jarFailed":    result.JarFailed,
		"error":        result.Error,
	})
}

// CheckAllHealth Handler: 检查所有源的健康状态
func CheckAllHealth(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
		Force bool `json:"force"`
	}
	c.ShouldBindJSON(&req)

	type HealthResult struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		HealthScore  int    `json:"healthScore"`
		HealthStatus string `json:"healthStatus"`
		SiteTotal    int    `json:"siteTotal"`
		SiteCrawler  int    `json:"siteCrawler"`
		JarTotal     int    `json:"jarTotal"`
		JarSuccess   int    `json:"jarSuccess"`
		JarFailed    int    `json:"jarFailed"`
		Error        string `json:"error,omitempty"`
	}

	results := make([]HealthResult, 0)
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	idsToCheck := req.IDs
	if len(idsToCheck) == 0 {
		Mu.Lock()
		for _, src := range MemorySources {
			idsToCheck = append(idsToCheck, src.ID)
		}
		Mu.Unlock()
	}

	for _, id := range idsToCheck {
		Mu.Lock()
		var src *models.SourceItem
		for i := range MemorySources {
			if MemorySources[i].ID == id {
				src = &MemorySources[i]
				break
			}
		}
		Mu.Unlock()

		if src == nil {
			continue
		}

		wg.Add(1)
		go func(source *models.SourceItem) {
			defer wg.Done()

			result := CheckSourceHealth(source)

			Mu.Lock()
			for i := range MemorySources {
				if MemorySources[i].ID == source.ID {
					UpdateSourceHealth(&MemorySources[i], result)
					break
				}
			}
			Mu.Unlock()

			resultsMu.Lock()
			results = append(results, HealthResult{
				ID:           source.ID,
				Name:         source.Name,
				HealthScore:  result.HealthScore,
				HealthStatus: result.HealthStatus,
				SiteTotal:    result.SiteTotal,
				SiteCrawler:  result.SiteCrawler,
				JarTotal:     result.JarTotal,
				JarSuccess:   result.JarSuccess,
				JarFailed:    result.JarFailed,
				Error:        result.Error,
			})
			resultsMu.Unlock()
		}(src)
	}
	wg.Wait()

	Mu.Lock()
	saveSourcesToFile()
	Mu.Unlock()

	c.JSON(200, gin.H{
		"success": true,
		"results": results,
		"count":   len(results),
	})
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

// GetSchedule Handler: 获取计划任务配置
func GetSchedule(c *gin.Context) {
	Mu.Lock()
	defer Mu.Unlock()

	c.JSON(200, MemorySchedule)
}

// SaveSchedule Handler: 保存计划任务配置
func SaveSchedule(c *gin.Context) {
	var req models.ScheduleConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效输入"})
		return
	}

	Mu.Lock()
	MemorySchedule = req
	saveScheduleToFile()
	Mu.Unlock()

	StartScheduler()

	c.JSON(200, gin.H{"message": "保存成功"})
}

// GenerateMultiConfig Handler: 生成多仓配置
func GenerateMultiConfig(c *gin.Context) {
	Mu.Lock()
	if IsAggregating {
		Mu.Unlock()
		c.JSON(400, gin.H{"error": "正在聚合中，请稍后再试", "success": false})
		return
	}
	IsAggregating = true
	Mu.Unlock()

	defer func() {
		Mu.Lock()
		IsAggregating = false
		Mu.Unlock()
	}()

	type sourceWithSpeed struct {
		source models.SourceItem
		speed  int
		config *models.TVConfig
	}

	var validSources []sourceWithSpeed
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	for i := range MemorySources {
		if !MemorySources[i].Enabled {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			src := MemorySources[idx]
			
			if NeedHealthCheck(&src) {
				result := CheckSourceHealth(&src)
				Mu.Lock()
				for i := range MemorySources {
					if MemorySources[i].ID == src.ID {
						UpdateSourceHealth(&MemorySources[i], result)
						break
					}
				}
				Mu.Unlock()
			}
			
			config, responseTime, err := fetchAndParse(src.URL)
			if err == nil {
				if ShouldIncludeSource(&src, config) {
					resultsMu.Lock()
					validSources = append(validSources, sourceWithSpeed{source: src, speed: responseTime, config: config})
					resultsMu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()

	Mu.Lock()
	saveSourcesToFile()
	Mu.Unlock()

	for i := 0; i < len(validSources); i++ {
		for j := i + 1; j < len(validSources); j++ {
			if validSources[j].speed < validSources[i].speed {
				validSources[i], validSources[j] = validSources[j], validSources[i]
			}
		}
	}

	videoList := []models.VideoSource{}
	totalJars := 0
	for _, vs := range validSources {
		replaced, skipped := CacheSourceJars(vs.source.ID, vs.source.URL, vs.config)
		totalJars += replaced
		
		if skipped > 0 && vs.source.HealthStatus == "healthy" {
			log.Printf("多仓排除绿灯源(缺失jar): %s", vs.source.Name)
			continue
		}
		
		if skipped > 0 && vs.source.HealthStatus == "warning" {
			log.Printf("多仓黄灯源保留原始jar URL: %s", vs.source.Name)
		}
		
		saveSourceCache(vs.source.ID, vs.config)
		
		speedStr := fmt.Sprintf("%dms", vs.speed)
		name := fmt.Sprintf("🚀 %s (%s)", vs.source.Name, speedStr)
		videoList = append(videoList, models.VideoSource{
			Name: name,
			URL:  fmt.Sprintf("__HOST__/source/%d", vs.source.ID),
		})
	}

	Mu.Lock()
	MemoryMultiConfig = models.TVConfig{
		Urls:      videoList,
		VideoList: videoList,
	}
	saveMultiConfigToFile()
	Mu.Unlock()

	c.JSON(200, gin.H{
		"success":  true,
		"count":    len(videoList),
		"jarCount": totalJars,
		"message":  fmt.Sprintf("已生成多仓配置，共 %d 个源，缓存 %d 个jar文件", len(videoList), totalJars),
	})
}

// GenerateMultiConfigInternal 内部调用的多仓生成函数
func GenerateMultiConfigInternal() {
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

	type sourceWithSpeed struct {
		source models.SourceItem
		speed  int
		config *models.TVConfig
	}

	var validSources []sourceWithSpeed
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	for i := range MemorySources {
		if !MemorySources[i].Enabled {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			src := MemorySources[idx]
			
			if NeedHealthCheck(&src) {
				result := CheckSourceHealth(&src)
				Mu.Lock()
				for i := range MemorySources {
					if MemorySources[i].ID == src.ID {
						UpdateSourceHealth(&MemorySources[i], result)
						break
					}
				}
				Mu.Unlock()
			}
			
			var config *models.TVConfig
			var responseTime int
			var err error
			
			if MemoryGlobalConfig.MultiPreferLocal && src.Localized && src.LocalStatus == "success" {
				config, err = GetLocalConfig(src.ID)
				if err == nil {
					responseTime = 0
				} else {
					config, responseTime, err = fetchAndParse(src.URL)
				}
			} else {
				config, responseTime, err = fetchAndParse(src.URL)
			}
			
			if err == nil {
				if ShouldIncludeSource(&src, config) {
					resultsMu.Lock()
					validSources = append(validSources, sourceWithSpeed{source: src, speed: responseTime, config: config})
					resultsMu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()

	Mu.Lock()
	saveSourcesToFile()
	Mu.Unlock()

	for i := 0; i < len(validSources); i++ {
		for j := i + 1; j < len(validSources); j++ {
			if validSources[j].speed < validSources[i].speed {
				validSources[i], validSources[j] = validSources[j], validSources[i]
			}
		}
	}

	videoList := []models.VideoSource{}
	totalJars := 0
	for _, vs := range validSources {
		downloaded, _ := CacheSourceJars(vs.source.ID, vs.source.URL, vs.config)
		totalJars += downloaded
		saveSourceCache(vs.source.ID, vs.config)
		
		speedStr := fmt.Sprintf("%dms", vs.speed)
		name := fmt.Sprintf("🚀 %s (%s)", vs.source.Name, speedStr)
		if vs.speed == 0 && vs.source.Localized {
			name = fmt.Sprintf("🚀 %s (本地)", vs.source.Name)
		}
		videoList = append(videoList, models.VideoSource{
			Name: name,
			URL:  fmt.Sprintf("__HOST__/source/%d", vs.source.ID),
		})
	}

	Mu.Lock()
	MemoryMultiConfig = models.TVConfig{
		Urls:      videoList,
		VideoList: videoList,
	}
	saveMultiConfigToFile()
	Mu.Unlock()

	log.Printf("计划任务：已生成多仓配置，共 %d 个源，缓存 %d 个jar文件", len(videoList), totalJars)
}

// CheckAllHealthInternal 内部调用的健康检查函数
func CheckAllHealthInternal() {
	var wg sync.WaitGroup

	Mu.Lock()
	sources := make([]models.SourceItem, len(MemorySources))
	copy(sources, MemorySources)
	Mu.Unlock()

	for i := range sources {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			src := sources[idx]
			
			result := CheckSourceHealth(&src)
			
			Mu.Lock()
			for i := range MemorySources {
				if MemorySources[i].ID == src.ID {
					UpdateSourceHealth(&MemorySources[i], result)
					break
				}
			}
			Mu.Unlock()
		}(i)
	}
	wg.Wait()

	Mu.Lock()
	saveSourcesToFile()
	Mu.Unlock()

	log.Printf("计划任务：已完成所有源的健康检查")
}

// GetMultiConfig Handler: 返回多仓配置
func GetMultiConfig(c *gin.Context) {
	Mu.Lock()
	config := MemoryMultiConfig
	Mu.Unlock()

	if len(config.VideoList) == 0 {
		c.JSON(200, gin.H{
			"urls":      []models.VideoSource{},
			"videoList": []models.VideoSource{},
		})
		return
	}

	c.JSON(200, gin.H{
		"urls":      config.VideoList,
		"videoList": config.VideoList,
	})
}

// GetMultiConfigStatus Handler: 获取多仓配置状态
func GetMultiConfigStatus(c *gin.Context) {
	Mu.Lock()
	count := len(MemoryMultiConfig.VideoList)
	Mu.Unlock()

	c.JSON(200, gin.H{
		"count": count,
	})
}

type Category struct {
	TypeID   string `json:"type_id"`
	TypeName string `json:"type_name"`
}

func HandleHomeAPI(c *gin.Context) {
	Mu.Lock()
	homeMenuSourceID := MemoryHomeMenuSource
	sources := MemorySources
	Mu.Unlock()

	targetSourceID := homeMenuSourceID
	if targetSourceID == 0 {
		fastestTime := -1
		fastestID := 0
		for _, src := range sources {
			if !src.Enabled || src.LastStatus != "success" {
				continue
			}
			if fastestTime == -1 || src.ResponseTime < fastestTime {
				fastestTime = src.ResponseTime
				fastestID = src.ID
			}
		}
		targetSourceID = fastestID
	}

	var categories []Category
	if targetSourceID > 0 {
		cachePath := getSourceCachePath(targetSourceID)
		data, err := os.ReadFile(cachePath)
		if err == nil {
			var config models.TVConfig
			if err := json.Unmarshal(data, &config); err == nil {
				for _, site := range config.Sites {
					if site.Type == 0 || site.Type == 1 || site.Type == 3 {
						categories = append(categories, Category{
							TypeID:   site.Key,
							TypeName: site.Name,
						})
					}
				}
			}
		}
	}

	if len(categories) == 0 {
		categories = []Category{
			{TypeID: "movie", TypeName: "电影"},
			{TypeID: "tv", TypeName: "电视剧"},
			{TypeID: "variety", TypeName: "综艺"},
			{TypeID: "anime", TypeName: "动漫"},
			{TypeID: "documentary", TypeName: "纪录片"},
		}
	}

	c.JSON(200, gin.H{
		"class": categories,
	})
}

type MenuSourceItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ResponseTime int    `json:"responseTime"`
	IsFastest    bool   `json:"isFastest"`
	IsSelected   bool   `json:"isSelected"`
}

func GetHomeMenuSources(c *gin.Context) {
	Mu.Lock()
	defer Mu.Unlock()

	var sources []MenuSourceItem
	fastestTime := -1
	fastestID := 0

	for _, src := range MemorySources {
		if !src.Enabled || src.LastStatus != "success" {
			continue
		}
		if fastestTime == -1 || src.ResponseTime < fastestTime {
			fastestTime = src.ResponseTime
			fastestID = src.ID
		}
	}

	for _, src := range MemorySources {
		if !src.Enabled || src.LastStatus != "success" {
			continue
		}
		sources = append(sources, MenuSourceItem{
			ID:           src.ID,
			Name:         src.Name,
			ResponseTime: src.ResponseTime,
			IsFastest:    src.ID == fastestID,
			IsSelected:   src.ID == MemoryHomeMenuSource || (MemoryHomeMenuSource == 0 && src.ID == fastestID),
		})
	}

	c.JSON(200, gin.H{
		"sources":         sources,
		"currentSourceId": MemoryHomeMenuSource,
		"fastestId":       fastestID,
	})
}

func SetHomeMenuSource(c *gin.Context) {
	var req struct {
		SourceID int `json:"sourceId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效输入"})
		return
	}

	Mu.Lock()
	MemoryHomeMenuSource = req.SourceID
	saveHomeMenuSourceToFile()
	Mu.Unlock()

	c.JSON(200, gin.H{"message": "保存成功", "sourceId": req.SourceID})
}

// GetGlobalConfig Handler: 获取全局配置
func GetGlobalConfig(c *gin.Context) {
	Mu.Lock()
	defer Mu.Unlock()
	c.JSON(200, MemoryGlobalConfig)
}

// SaveGlobalConfig Handler: 保存全局配置
func SaveGlobalConfig(c *gin.Context) {
	var config models.GlobalConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{"error": "无效输入"})
		return
	}

	Mu.Lock()
	MemoryGlobalConfig = config
	
	// 同步更新其他配置
	MemoryFilterWords = config.FilterWords
	MemorySchedule.Enabled = config.ScheduleEnabled
	MemorySchedule.Mode = config.AggMode
	MemorySchedule.MaxSites = config.MaxSites
	MemoryHomeMenuSource = config.HomeMenuSource
	
	saveGlobalConfigToFile()
	saveFilterWordsToFile()
	saveScheduleToFile()
	saveHomeMenuSourceToFile()
	Mu.Unlock()

	// 重新启动计划任务调度器
	StartScheduler()

	c.JSON(200, gin.H{"message": "保存成功"})
}

// LocalizeSourceHandler Handler: 本地化单个源
func LocalizeSourceHandler(c *gin.Context) {
	idStr := c.Param("id")
	sourceID := 0
	fmt.Sscanf(idStr, "%d", &sourceID)

	if sourceID == 0 {
		c.JSON(400, gin.H{"error": "无效的源ID"})
		return
	}

	go func() {
		err := LocalizeSource(sourceID)
		if err != nil {
			log.Printf("本地化失败: sourceID=%d, error=%v", sourceID, err)
		}
	}()

	c.JSON(200, gin.H{"message": "开始本地化"})
}

// BatchLocalizeHandler Handler: 批量本地化
func BatchLocalizeHandler(c *gin.Context) {
	Mu.Lock()
	var greenUnlocalized []int
	for _, s := range MemorySources {
		if s.Enabled && s.HealthStatus == "healthy" && !s.Localized {
			greenUnlocalized = append(greenUnlocalized, s.ID)
		}
	}
	Mu.Unlock()

	if len(greenUnlocalized) == 0 {
		c.JSON(200, gin.H{"message": "没有需要本地化的绿色源"})
		return
	}

	successCount := 0
	failCount := 0

	for _, id := range greenUnlocalized {
		err := LocalizeSource(id)
		if err != nil {
			failCount++
		} else {
			successCount++
		}
	}

	c.JSON(200, gin.H{
		"message":      fmt.Sprintf("批量本地化完成：成功 %d 个，失败 %d 个", successCount, failCount),
		"success":      successCount,
		"failed":        failCount,
	})
}

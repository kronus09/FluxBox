package api

import (
	"FluxBox/models"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const HealthCheckCacheHours = 24

type HealthCheckResult struct {
	SourceID      int
	HealthScore   int
	HealthStatus  string
	SiteTotal     int
	SiteCrawler   int
	SiteCollector int
	JarTotal      int
	JarSuccess    int
	JarFailed     int
	JarCached     int
	Error         string
}

func CheckSourceHealth(src *models.SourceItem) *HealthCheckResult {
	result := &HealthCheckResult{
		SourceID: src.ID,
	}

	config, _, err := fetchAndParse(src.URL)
	if err != nil {
		result.HealthScore = 0
		result.HealthStatus = "failed"
		result.Error = err.Error()
		return result
	}

	result.SiteTotal = len(config.Sites)
	for _, site := range config.Sites {
		if site.Type == 3 {
			result.SiteCrawler++
		} else if site.Type == 0 || site.Type == 1 {
			result.SiteCollector++
		}
	}

	jarURLs := make(map[string]bool)

	if config.Spider != "" {
		jarURL := resolveJarURL(src.URL, config.Spider)
		if jarURL != "" {
			jarURLs[jarURL] = true
		}
	}

	for _, site := range config.Sites {
		if site.Type == 3 && site.Jar != "" {
			jarURL := resolveJarURL(src.URL, site.Jar)
			if jarURL != "" {
				jarURLs[jarURL] = true
			}
		}
	}

	result.JarTotal = len(jarURLs)

	if result.SiteCrawler == 0 && len(jarURLs) == 0 {
		result.HealthScore = 100
		result.HealthStatus = "healthy"
		return result
	}

	for jarURL := range jarURLs {
		jarName := extractJarName(jarURL)
		localPath := filepath.Join(getJarCacheDir(src.ID), jarName)

		success, cached := downloadJarWithMD5(jarURL, localPath, src.ID)
		if success {
			result.JarSuccess++
			if cached {
				result.JarCached++
			}
		} else {
			log.Printf("健康检查: jar下载失败 sourceID=%d jar=%s", src.ID, jarName)
			result.JarFailed++
		}
	}

	result.HealthScore = 100

	if config.Spider != "" && result.JarFailed > 0 {
		spiderJarURL := resolveJarURL(src.URL, config.Spider)
		if spiderJarURL != "" {
			spiderJarName := extractJarName(config.Spider)
			localPath := filepath.Join(getJarCacheDir(src.ID), spiderJarName)
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				result.HealthScore -= 40
			} else {
				result.HealthScore -= 10
			}
		}
	}

	if result.JarTotal > 0 && result.JarFailed > 0 {
		failRate := float64(result.JarFailed) / float64(result.JarTotal)
		if failRate > 0.5 {
			result.HealthScore -= 20
		} else {
			result.HealthScore -= 10
		}
	}

	if result.HealthScore >= 80 {
		result.HealthStatus = "healthy"
	} else if result.HealthScore >= 60 {
		result.HealthStatus = "warning"
	} else if result.HealthScore >= 30 {
		result.HealthStatus = "unhealthy"
	} else {
		result.HealthStatus = "failed"
	}

	return result
}

func downloadJarWithMD5(jarURL, localPath string, sourceID int) (success bool, cached bool) {
	if _, err := os.Stat(localPath); err == nil {
		md5Path := localPath + ".md5"
		if md5Data, err := os.ReadFile(md5Path); err == nil {
			localMD5 := strings.TrimSpace(string(md5Data))

			remoteMD5, err := getRemoteJarMD5(jarURL)
			if err == nil && remoteMD5 != "" && remoteMD5 == localMD5 {
				log.Printf("jar MD5匹配，跳过下载: %s", localPath)
				return true, true
			}
			if err != nil || remoteMD5 == "" {
				log.Printf("jar已存在，跳过下载: %s", localPath)
				return true, true
			}
		} else {
			log.Printf("jar已存在，跳过下载: %s", localPath)
			return true, true
		}
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("创建目录失败: %v", err)
		if _, err := os.Stat(localPath); err == nil {
			log.Printf("使用现有缓存: %s", localPath)
			return true, true
		}
		return false, false
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, _ := http.NewRequest("GET", jarURL, nil)
	req.Header.Set("User-Agent", "okhttp/3.15.0")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("下载失败: %v", err)
		if _, err := os.Stat(localPath); err == nil {
			log.Printf("使用现有缓存: %s", localPath)
			return true, true
		}
		return false, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("HTTP错误: %d", resp.StatusCode)
		if _, err := os.Stat(localPath); err == nil {
			log.Printf("使用现有缓存: %s", localPath)
			return true, true
		}
		return false, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取响应失败: %v", err)
		if _, err := os.Stat(localPath); err == nil {
			log.Printf("使用现有缓存: %s", localPath)
			return true, true
		}
		return false, false
	}

	oldBakPath := localPath + ".bak"
	os.Remove(oldBakPath)
	oldMD5BakPath := localPath + ".md5.bak"
	os.Remove(oldMD5BakPath)

	if _, err := os.Stat(localPath); err == nil {
		bakPath := localPath + ".bak"
		if err := os.Rename(localPath, bakPath); err != nil {
			log.Printf("备份旧jar失败: %v", err)
		} else {
			log.Printf("已备份旧jar: %s", bakPath)
		}

		oldMD5Path := localPath + ".md5"
		if _, err := os.Stat(oldMD5Path); err == nil {
			os.Rename(oldMD5Path, oldMD5Path+".bak")
		}
	}

	if err := os.WriteFile(localPath, body, 0644); err != nil {
		log.Printf("写入文件失败: %v", err)
		if _, err := os.Stat(localPath); err == nil {
			log.Printf("使用现有缓存: %s", localPath)
			return true, true
		}
		return false, false
	}

	hash := md5.Sum(body)
	md5Str := hex.EncodeToString(hash[:])
	md5Path := localPath + ".md5"
	os.WriteFile(md5Path, []byte(md5Str), 0644)

	if isImageFile(localPath) || DetectImageJar(body) {
		if err := decryptImageJar(localPath); err != nil {
			log.Printf("图片jar解密失败: %s, error: %v", localPath, err)
			os.Remove(localPath)
			if _, err := os.Stat(localPath); err == nil {
				log.Printf("使用现有缓存: %s", localPath)
				return true, true
			}
			return false, false
		}
		localPath = strings.TrimSuffix(localPath, filepath.Ext(localPath)) + ".jar"
	}

	log.Printf("jar下载成功: %s (MD5: %s)", localPath, md5Str[:8])
	return true, false
}

func getRemoteJarMD5(jarURL string) (string, error) {
	if strings.Contains(jarURL, ";md5;") {
		parts := strings.Split(jarURL, ";md5;")
		if len(parts) > 1 {
			return strings.Split(parts[1], ";")[0], nil
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("HEAD", jarURL, nil)
	req.Header.Set("User-Agent", "okhttp/3.15.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	contentMD5 := resp.Header.Get("Content-MD5")
	if contentMD5 != "" {
		return contentMD5, nil
	}

	etag := resp.Header.Get("ETag")
	if etag != "" {
		etag = strings.Trim(etag, `"`)
		return etag, nil
	}

	return "", fmt.Errorf("no MD5 available")
}

func NeedHealthCheck(src *models.SourceItem) bool {
	if src.LastCheckTime == "" {
		return true
	}

	lastCheck, err := time.Parse(time.RFC3339, src.LastCheckTime)
	if err != nil {
		return true
	}

	return time.Since(lastCheck) > time.Duration(HealthCheckCacheHours)*time.Hour
}

func UpdateSourceHealth(src *models.SourceItem, result *HealthCheckResult) {
	src.HealthScore = result.HealthScore
	src.HealthStatus = result.HealthStatus
	src.SiteTotal = result.SiteTotal
	src.SiteCrawler = result.SiteCrawler
	src.SiteCollector = result.SiteCollector
	src.JarTotal = result.JarTotal
	src.JarSuccess = result.JarSuccess
	src.JarFailed = result.JarFailed
	src.JarCached = result.JarCached
	src.LastCheckTime = time.Now().Format(time.RFC3339)

	// 优先本地化模式下：本地化源只更新健康状态，但永远保持启用
	if MemoryGlobalConfig.MultiPreferLocal && src.Localized && src.LocalStatus == "success" {
		if !src.Enabled {
			src.Enabled = true
			log.Printf("本地化源 %s 已自动启用 (健康状态:%s)", src.Name, result.HealthStatus)
		}
		log.Printf("本地化源 %s 健康状态更新: %s(%d分)", src.Name, result.HealthStatus, result.HealthScore)
		return
	}

	// 健康的源不应该被自动禁用，直接返回
	if result.HealthStatus == "healthy" {
		// healthy的源系统不会自动禁用 → 不做自动启用
		// （尊重用户手动禁用的选择）
		return
	}

	// 健康恢复：只有之前符合自动禁用条件的源才自动恢复
	// 这样不会干扰用户手动禁用的源
	if !src.Enabled {
		shouldDisableNow := false
		if result.HealthStatus == "unhealthy" && MemoryGlobalConfig.AutoDisableUnhealthy {
			shouldDisableNow = true
		}
		if result.HealthStatus == "warning" && MemoryGlobalConfig.AutoDisableWarning {
			shouldDisableNow = true
		}
		if result.HealthStatus == "failed" && MemoryGlobalConfig.AutoDisableFailed {
			shouldDisableNow = true
		}
		if !shouldDisableNow {
			// ✅ 关键逻辑：
			// 这个状态本来【应该被自动禁用】，现在不需要了
			// → 说明是之前被系统自动禁用的，现在健康恢复了，自动启用
			src.Enabled = true
			log.Printf("源 %s 健康恢复为%s(%d分)，已自动重新启用", src.Name, result.HealthStatus, result.HealthScore)
			return
		}
	}

	// 根据全局配置决定是否自动禁用
	shouldDisable := false
	// 优先本地化模式下，本地化源不自动禁用
	localizedProtected := MemoryGlobalConfig.MultiPreferLocal && src.Localized && src.LocalStatus == "success"
	if !localizedProtected {
		if result.HealthStatus == "unhealthy" && MemoryGlobalConfig.AutoDisableUnhealthy {
			shouldDisable = true
		}
		if result.HealthStatus == "warning" && MemoryGlobalConfig.AutoDisableWarning {
			shouldDisable = true
		}
		if result.HealthStatus == "failed" && MemoryGlobalConfig.AutoDisableFailed {
			shouldDisable = true
		}
	}

	if shouldDisable && src.Enabled {
		src.Enabled = false
		log.Printf("源 %s 健康状态为%s(%d分)，已自动禁用", src.Name, result.HealthStatus, result.HealthScore)
	}
}

func ShouldIncludeSource(src *models.SourceItem, config *models.TVConfig) bool {
	// 本地化成功的源直接纳入，跳过其他检查
	if MemoryGlobalConfig.MultiPreferLocal && src.Localized && src.LocalStatus == "success" {
		return true
	}

	if !src.Enabled {
		return false
	}

	// 根据全局配置决定是否包含警告源
	if src.HealthStatus == "warning" {
		if !MemoryGlobalConfig.MultiIncludeWarning {
			log.Printf("多仓排除警告源: %s (健康度:%d)", src.Name, src.HealthScore)
			return false
		}
	}

	if src.HealthStatus == "unhealthy" || src.HealthStatus == "failed" {
		log.Printf("多仓排除源: %s (健康度:%d, 状态:%s)", src.Name, src.HealthScore, src.HealthStatus)
		return false
	}

	return true
}

func ShouldIncludeSite(site *models.Site, src *models.SourceItem, config *models.TVConfig) bool {
	if site.Type == 0 || site.Type == 1 {
		return true
	}

	if site.Type == 3 {
		jarPath := site.Jar
		if jarPath == "" && config.Spider != "" {
			jarPath = config.Spider
		}

		if jarPath == "" {
			return true
		}

		jarURL := resolveJarURL(src.URL, jarPath)
		if jarURL == "" {
			return false
		}

		jarName := extractJarName(jarURL)
		localPath := filepath.Join(getJarCacheDir(src.ID), jarName)

		if _, err := os.Stat(localPath); err == nil {
			return true
		}

		log.Printf("站点jar不存在: site=%s jar=%s", site.Name, jarName)
		return false
	}

	return true
}

func GetHealthStatusIcon(status string) string {
	switch status {
	case "healthy":
		return "🟢"
	case "warning":
		return "🟡"
	case "unhealthy":
		return "🔴"
	case "failed":
		return "⚫"
	default:
		return "⚪"
	}
}

func GetHealthStatusText(status string) string {
	switch status {
	case "healthy":
		return "健康"
	case "warning":
		return "一般"
	case "unhealthy":
		return "不健康"
	case "failed":
		return "失效"
	default:
		return "未检查"
	}
}

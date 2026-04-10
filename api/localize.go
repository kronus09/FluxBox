package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"FluxBox/models"
)

const LocalSourceDir = "data/local_sources/"

func getLocalSourceDir(sourceID int) string {
	return fmt.Sprintf("%s%d/", LocalSourceDir, sourceID)
}

func getLocalAssetDir(sourceID int) string {
	return fmt.Sprintf("%s%d/assets/", LocalSourceDir, sourceID)
}

func getLocalConfigPath(sourceID int) string {
	return fmt.Sprintf("%s%d/config.json", LocalSourceDir, sourceID)
}

func containsFilterWord(str string) bool {
	Mu.Lock()
	words := MemoryFilterWords
	Mu.Unlock()
	strLower := strings.ToLower(str)
	for _, word := range words {
		if strings.Contains(strLower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func LocalizeSource(sourceID int) error {
	Mu.Lock()
	var source *models.SourceItem
	for i := range MemorySources {
		if MemorySources[i].ID == sourceID {
			source = &MemorySources[i]
			break
		}
	}
	Mu.Unlock()

	if source == nil {
		return fmt.Errorf("源不存在")
	}

	if source.HealthStatus != "healthy" {
		return fmt.Errorf("需绿色源才能本地化")
	}

	log.Printf("开始本地化: [%d] %s", sourceID, source.Name)

	// 先清理旧的本地化文件，保证每次都是干干净净重新开始
	DeleteLocalSource(sourceID)

	// 强制重置状态，不管之前是pending还是failed，都可以重新开始
	source.Localized = false
	source.LocalStatus = ""
	source.LocalError = ""
	saveSourcesToFile()

	source.Localized = true
	source.LocalStatus = "pending"
	source.LocalError = ""
	saveSourcesToFile()

	config, _, err := fetchAndParse(source.URL)
	if err != nil {
		source.Localized = true
		source.LocalStatus = "failed"
		source.LocalError = fmt.Sprintf("获取配置失败: %v", err)
		saveSourcesToFile()
		log.Printf("本地化失败: [%d] %s, 获取配置失败: %v", sourceID, source.Name, err)
		return err
	}

	localDir := getLocalSourceDir(sourceID)
	assetDir := getLocalAssetDir(sourceID)
	os.MkdirAll(localDir, 0755)
	os.MkdirAll(assetDir+"jars/", 0755)
	os.MkdirAll(assetDir+"js/", 0755)
	os.MkdirAll(assetDir+"json/", 0755)

	downloaded := 0
	realFailed := 0
	var warnings []string
	var filteredSkipped []string

	if config.Spider != "" {
		jarURL := resolveAssetURL(source.URL, config.Spider)
		if jarURL != "" {
			localPath := filepath.Join(assetDir, "jars", extractAssetName(jarURL))
			if err := downloadAsset(jarURL, localPath); err != nil {
				if containsFilterWord(jarURL) || containsFilterWord(config.Spider) {
					filteredSkipped = append(filteredSkipped, fmt.Sprintf("spider jar(过滤词)"))
				} else {
					realFailed++
					warnings = append(warnings, fmt.Sprintf("spider jar: %v", err))
				}
			} else {
				downloaded++
				config.Spider = fmt.Sprintf("__HOST__/asset/%d/jars/%s", sourceID, extractAssetName(jarURL))
			}
		}
	}

	for i := range config.Sites {
		if config.Sites[i].Jar == "" && config.Spider != "" {
			config.Sites[i].Jar = config.Spider
		}
	}

	type jsAsset struct {
		siteIndex int
		url       string
		localPath string
		isApi     bool
		assetType string
		extMap    map[string]interface{}
	}

	var jsAssets []jsAsset
	jsCount := 0

	for i := range config.Sites {
		site := &config.Sites[i]

		if site.Jar != "" {
			jarURL := resolveAssetURL(source.URL, site.Jar)
			if jarURL != "" {
				site.Jar = jarURL
			}
		}

		if models.EqInt(site.Type, 3) && isJSURL(site.Api) {
			jsURL := resolveAssetURL(source.URL, site.Api)
			if jsURL != "" {
				jsCount++
				jsAssets = append(jsAssets, jsAsset{
					siteIndex: i,
					url:       jsURL,
					localPath: filepath.Join(assetDir, "js", extractAssetName(jsURL)),
					isApi:     true,
					assetType: "js",
				})
			}
		}

		if site.Ext != nil {
			extStr := ""
			var extMap map[string]interface{}
			switch v := site.Ext.(type) {
			case string:
				extStr = v
			case map[string]interface{}:
				extMap = v
				if urlVal, ok := v["url"].(string); ok {
					extStr = urlVal
				}
			}

			if extStr != "" && (isJSURL(extStr) || isJSONURL(extStr)) {
				extURL := resolveAssetURL(source.URL, extStr)
				if extURL != "" {
					assetType := "js"
					if isJSONURL(extStr) {
						assetType = "json"
					}
					jsCount++
					jsAssets = append(jsAssets, jsAsset{
						siteIndex: i,
						url:       extURL,
						localPath: filepath.Join(assetDir, assetType, extractAssetName(extURL)),
						isApi:     false,
						assetType: assetType,
						extMap:    extMap,
					})
				}
			}
		}
	}

	if jsCount > 10 {
		log.Printf("检测到JS/JSON资源 %d 个 (>10)，跳过下载仅补全URL", jsCount)
		for _, asset := range jsAssets {
			site := &config.Sites[asset.siteIndex]
			if asset.isApi {
				site.Api = asset.url
			} else {
				if asset.extMap != nil {
					asset.extMap["url"] = asset.url
					site.Ext = asset.extMap
				} else {
					site.Ext = asset.url
				}
			}
		}
	} else {
		for _, asset := range jsAssets {
			site := &config.Sites[asset.siteIndex]
			siteName := site.Name
			if containsFilterWord(siteName) || containsFilterWord(asset.url) {
				filteredSkipped = append(filteredSkipped, fmt.Sprintf("site %s %s(过滤词)", siteName, asset.assetType))
				if asset.isApi {
					site.Api = asset.url
				} else {
					if asset.extMap != nil {
						asset.extMap["url"] = asset.url
						site.Ext = asset.extMap
					} else {
						site.Ext = asset.url
					}
				}
				continue
			}

			if err := downloadAsset(asset.url, asset.localPath); err != nil {
				realFailed++
				warnings = append(warnings, fmt.Sprintf("site %s %s: %v", siteName, asset.assetType, err))
				if asset.isApi {
					site.Api = asset.url
				} else {
					if asset.extMap != nil {
						asset.extMap["url"] = asset.url
						site.Ext = asset.extMap
					} else {
						site.Ext = asset.url
					}
				}
			} else {
				downloaded++
				newURL := fmt.Sprintf("__HOST__/asset/%d/%s/%s", sourceID, asset.assetType, filepath.Base(asset.localPath))
				if asset.isApi {
					site.Api = newURL
				} else {
					if asset.extMap != nil {
						asset.extMap["url"] = newURL
						site.Ext = asset.extMap
					} else {
						site.Ext = newURL
					}
				}
			}
		}
	}

	configPath := getLocalConfigPath(sourceID)
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		source.Localized = true
		source.LocalStatus = "failed"
		source.LocalError = fmt.Sprintf("保存配置失败: %v", err)
		saveSourcesToFile()
		return err
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		source.Localized = true
		source.LocalStatus = "failed"
		source.LocalError = fmt.Sprintf("保存配置失败: %v", err)
		saveSourcesToFile()
		return err
	}

	if realFailed >= 3 {
		source.Localized = true
		source.LocalStatus = "failed"
		source.LocalError = fmt.Sprintf("下载失败%d个: %s", realFailed, strings.Join(warnings, "; "))
		saveSourcesToFile()
		return fmt.Errorf(source.LocalError)
	}

	source.Localized = true
	source.LocalStatus = "success"
	source.LocalTime = time.Now().Format("2006-01-02")

	if len(warnings) > 0 || len(filteredSkipped) > 0 {
		var notes []string
		if len(warnings) > 0 {
			notes = append(notes, fmt.Sprintf("警告%d个", len(warnings)))
		}
		if len(filteredSkipped) > 0 {
			notes = append(notes, fmt.Sprintf("跳过过滤词%d个", len(filteredSkipped)))
		}
		source.LocalError = strings.Join(notes, ", ")
	} else {
		source.LocalError = ""
	}
	saveSourcesToFile()

	log.Printf("本地化完成: [%d] %s, 成功下载=%d, 失败=%d, 过滤跳过=%d", sourceID, source.Name, downloaded, realFailed, len(filteredSkipped))
	return nil
}

func GetLocalConfig(sourceID int) (*models.TVConfig, error) {
	configPath := getLocalConfigPath(sourceID)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("本地配置不存在")
	}

	var config models.TVConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析本地配置失败")
	}

	return &config, nil
}

func DeleteLocalSource(sourceID int) error {
	localDir := getLocalSourceDir(sourceID)
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(localDir)
}

func resolveAssetURL(sourceURL, assetPath string) string {
	assetPath = strings.Split(assetPath, ";")[0]
	assetPath = strings.Split(assetPath, "?")[0]

	if strings.HasPrefix(assetPath, "http://") || strings.HasPrefix(assetPath, "https://") {
		return assetPath
	}

	u, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}

	base := path.Dir(u.Path)
	assetPath = strings.TrimPrefix(assetPath, "./")

	u.Path = path.Join(base, assetPath)
	return u.String()
}

func extractAssetName(assetURL string) string {
	assetURL = strings.Split(assetURL, ";")[0]
	assetURL = strings.Split(assetURL, "?")[0]
	_, name := path.Split(assetURL)
	name, _ = url.PathUnescape(name)
	if name == "" {
		name = "asset"
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

func isJSURL(url string) bool {
	url = strings.ToLower(url)
	return strings.HasSuffix(url, ".js") || strings.Contains(url, ".js?")
}

func isJSONURL(url string) bool {
	url = strings.ToLower(url)
	return strings.HasSuffix(url, ".json") || strings.Contains(url, ".json?")
}

func downloadAsset(url string, localPath string) error {
	log.Printf("  下载资源: %s", filepath.Base(localPath))
	client := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "okhttp/3.15.0")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return fmt.Errorf("空文件")
	}

	return os.WriteFile(localPath, data, 0644)
}

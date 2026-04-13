package api

import (
	"FluxBox/models"
	"FluxBox/parser"
	"bytes"
	"encoding/base64"
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
)

const JarCacheDir = "data/jars/"

func getJarCacheDir(sourceID int) string {
	return fmt.Sprintf("%s%d/", JarCacheDir, sourceID)
}

func getJarServiceURL(sourceID int, jarName string) string {
	encodedName := url.PathEscape(jarName)
	return fmt.Sprintf("__HOST__/jar/%d/%s", sourceID, encodedName)
}

func extractJarName(jarURL string) string {
	jarURL = strings.Split(jarURL, ";")[0]
	jarURL = strings.Split(jarURL, "?")[0]
	_, name := path.Split(jarURL)
	name, _ = url.PathUnescape(name)
	if name == "" {
		name = "spider.jar"
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if !strings.HasSuffix(strings.ToLower(name), ".jar") {
		name = name + ".jar"
	}
	return name
}

func resolveJarURL(sourceURL, jarPath string) string {
	jarPath = strings.Split(jarPath, ";")[0]
	jarPath = strings.Split(jarPath, "?")[0]

	if strings.HasPrefix(jarPath, "http://") || strings.HasPrefix(jarPath, "https://") {
		return jarPath
	}

	u, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}

	base := path.Dir(u.Path)

	jarPath = strings.TrimPrefix(jarPath, "./")

	u.Path = path.Join(base, jarPath)
	return u.String()
}

func isImageFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif"
}

func decryptImageJar(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	jsonStr, err := parser.ParseConfig(string(data))
	if err != nil {
		return err
	}

	decoded, err := base64.StdEncoding.DecodeString(jsonStr)
	if err == nil {
		jsonStr = string(decoded)
	}

	if !strings.HasPrefix(jsonStr, "PK") {
		return fmt.Errorf("not a valid jar file after decryption")
	}

	newPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".jar"
	if err := os.WriteFile(newPath, []byte(jsonStr), 0644); err != nil {
		return err
	}

	os.Remove(filePath)
	return nil
}

func downloadJar(jarURL, localPath string) error {
	if _, err := os.Stat(localPath); err == nil {
		return nil
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	client := GetHTTPClient(60 * time.Second)
	req, _ := http.NewRequest("GET", jarURL, nil)
	req.Header.Set("User-Agent", "okhttp/3.15.0")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := os.WriteFile(localPath, body, 0644); err != nil {
		return err
	}

	if isImageFile(localPath) || DetectImageJar(body) {
		if err := decryptImageJar(localPath); err != nil {
			log.Printf("图片jar解密失败: %s, error: %v", localPath, err)
			os.Remove(localPath)
			return err
		}
		localPath = strings.TrimSuffix(localPath, filepath.Ext(localPath)) + ".jar"
	}

	log.Printf("jar缓存成功: %s", localPath)
	return nil
}

func CacheSourceJars(sourceID int, sourceURL string, config *models.TVConfig) (int, int) {
	jarDir := getJarCacheDir(sourceID)
	replaced := 0
	skipped := 0

	if config.Spider != "" {
		if strings.Contains(config.Spider, "/asset/") {
			replaced++
		} else {
			jarURL := resolveJarURL(sourceURL, config.Spider)
			if jarURL != "" {
				jarName := extractJarName(jarURL)
				localPath := filepath.Join(jarDir, jarName)

				if _, err := os.Stat(localPath); err == nil {
					config.Spider = getJarServiceURL(sourceID, jarName)
					replaced++
				} else {
					skipped++
				}
			}
		}
	}

	for i := range config.Sites {
		if config.Sites[i].Jar != "" {
			if strings.Contains(config.Sites[i].Jar, "/asset/") {
				replaced++
			} else {
				jarURL := resolveJarURL(sourceURL, config.Sites[i].Jar)
				if jarURL != "" {
					jarName := extractJarName(jarURL)
					localPath := filepath.Join(jarDir, jarName)

					if _, err := os.Stat(localPath); err == nil {
						config.Sites[i].Jar = getJarServiceURL(sourceID, jarName)
						replaced++
					} else {
						skipped++
					}
				}
			}
		}
	}

	return replaced, skipped
}

func ClearSourceJarCache(sourceID int) error {
	jarDir := getJarCacheDir(sourceID)
	if _, err := os.Stat(jarDir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(jarDir)
}

func GetCachedJarCount(sourceID int) int {
	jarDir := getJarCacheDir(sourceID)
	entries, err := os.ReadDir(jarDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			count++
		}
	}
	return count
}

func CleanOldJars(sourceID int, validJarNames map[string]bool) {
	jarDir := getJarCacheDir(sourceID)
	entries, err := os.ReadDir(jarDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if !validJarNames[name] {
				os.Remove(filepath.Join(jarDir, name))
				log.Printf("清理旧jar: %s/%s", jarDir, name)
			}
		}
	}
}

func DetectImageJar(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return true
	}
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) {
		return true
	}
	if bytes.HasPrefix(data, []byte{0x47, 0x49, 0x46}) {
		return true
	}
	return false
}

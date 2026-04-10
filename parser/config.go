package parser

import (
	"bytes"
	"encoding/base64"
	"log"
	"regexp"
	"strings"
)

func DetectImage(data []byte) bool {
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

// CleanJSON 核心清洗逻辑：对付非标准、带注释、带乱码的 JSON
func CleanJSON(s string) string {
	s = strings.TrimPrefix(s, "\xef\xbb\xbf")

	start := strings.IndexAny(s, "{[")
	end := strings.LastIndexAny(s, "}]")
	if start == -1 || end == -1 || end <= start {
		return s
	}
	content := s[start : end+1]

	reMultiComments := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	content = reMultiComments.ReplaceAllString(content, "")

	reComments := regexp.MustCompile(`(?m)^[ \t]*//.*$`)
	content = reComments.ReplaceAllString(content, "")

	reHashComments := regexp.MustCompile(`(?m)^[ \t]*#.*$`)
	content = reHashComments.ReplaceAllString(content, "")

	reDirty := regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
	content = reDirty.ReplaceAllString(content, "")

	reMultiComma := regexp.MustCompile(`,(\s*,)+`)
	content = reMultiComma.ReplaceAllString(content, ",")

	reComma := regexp.MustCompile(`,(\s*[}\]])`)
	content = reComma.ReplaceAllString(content, "$1")

	return content
}

func IsHexString(s string) bool {
	if len(s) < 32 || len(s)%2 != 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func ParseConfig(rawContent string) (string, error) {
	rawContent = strings.TrimSpace(rawContent)
	if rawContent == "" {
		return "", nil
	}

	log.Println("  [解析流水线] 开始处理，长度:", len(rawContent))

	result, chain, err := ParsePipeline(rawContent)

	if len(chain) > 0 {
		log.Printf("  [解析流水线] 成功! 解析链: %v (%d层)", chain, len(chain))
	} else {
		log.Println("  [解析流水线] 使用标准JSON解析")
	}

	if !stringsContains(result, "{") {
		log.Println("  [解析流水线] 尝试Base64兜底解析...")
		decoded, base64Err := base64.StdEncoding.DecodeString(result)
		if base64Err == nil {
			if stringsContains(string(decoded), "{") {
				return CleanJSON(string(decoded)), nil
			}
		}
	}

	return CleanJSON(result), err
}

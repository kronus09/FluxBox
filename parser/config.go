package parser

import (
	"FluxBox/utils"
	"encoding/base64"
	"regexp"
	"strings"
)

// CleanJSON 核心清洗逻辑：对付非标准、带注释、带乱码的 JSON
func CleanJSON(s string) string {
	// 1. 移除 UTF-8 BOM
	s = strings.TrimPrefix(s, "\xef\xbb\xbf")

	// 2. 寻找物理边界
	start := strings.IndexAny(s, "{[")
	end := strings.LastIndexAny(s, "}]")
	if start == -1 || end == -1 || end <= start {
		return s
	}
	content := s[start : end+1]

	// 3. 移除 JS 风格的多行注释 (/* ... */)
	// 很多影视Box源使用多行注释作为分割线
	reMultiComments := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	content = reMultiComments.ReplaceAllString(content, "")

	// 4. 移除 JS 风格的单行注释 (// ...)
	// 只匹配行首的注释，避免误匹配 URL 中的 //
	reComments := regexp.MustCompile(`(?m)^[ \t]*//.*$`)
	content = reComments.ReplaceAllString(content, "")

	// 5. 移除 # 风格的注释行 (常见于配置文件)
	// 例如: # 这是注释 或 ## 标题
	reHashComments := regexp.MustCompile(`(?m)^[ \t]*#.*$`)
	content = reHashComments.ReplaceAllString(content, "")

	// 6. 移除多余的控制字符 (保持换行和空格，杀掉 00-08 等非法字节)
	reDirty := regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
	content = reDirty.ReplaceAllString(content, "")

	// 7. 修复连续逗号 (注释移除后可能产生 ,, 或 , \n ,)
	// 例如: [a, // comment\n, b] -> [a, , b] -> [a, b]
	reMultiComma := regexp.MustCompile(`,(\s*,)+`)
	content = reMultiComma.ReplaceAllString(content, ",")

	// 8. 修复末尾逗号 (例如 {"a":1,} -> {"a":1} 或 [1,] -> [1])
	reComma := regexp.MustCompile(`,(\s*[}\]])`)
	content = reComma.ReplaceAllString(content, "$1")

	return content
}

func ParseConfig(rawContent string) (string, error) {
	rawContent = strings.TrimSpace(rawContent)
	if rawContent == "" {
		return "", nil
	}

	// 0. 如果已经是标准 JSON 格式，直接清理返回
	// 避免后续逻辑误处理 JSON 内部的 ** 等字符
	if strings.HasPrefix(rawContent, "{") || strings.HasPrefix(rawContent, "[") {
		return CleanJSON(rawContent), nil
	}

	// 1. 图片隐写/偏移量提取
	if !strings.HasPrefix(rawContent, "**") && strings.Contains(rawContent, "**") {
		index := strings.Index(rawContent, "**")
		return ParseConfig(rawContent[index:])
	}

	// 2. 标准 AES-CBC (**密文[key;iv])
	reCBC := regexp.MustCompile(`\*\*(.*)\[((.*);(.*))\]`)
	if matches := reCBC.FindStringSubmatch(rawContent); len(matches) == 5 {
		res, err := utils.DecryptCBC(matches[1], matches[3], matches[4])
		if err == nil {
			return CleanJSON(res), nil
		}
	}

	// 3. 标准 AES-ECB (**密文[key])
	reECB := regexp.MustCompile(`\*\*(.*)\[(.*)\]`)
	if matchesECB := reECB.FindStringSubmatch(rawContent); len(matchesECB) == 3 {
		res, err := utils.DecryptECB(matchesECB[1], matchesECB[2])
		if err == nil {
			return CleanJSON(res), nil
		}
	}

	// 4. 处理饭太硬式的 **Base64 (递归剥洋葱)
	if strings.HasPrefix(rawContent, "**") {
		content := strings.TrimPrefix(rawContent, "**")
		if idx := strings.Index(content, "["); idx != -1 {
			content = content[:idx]
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err == nil {
			return ParseConfig(string(decoded))
		}
	}

	// 5. 纯 Base64 处理
	if !strings.HasPrefix(rawContent, "{") && !strings.HasPrefix(rawContent, "[") {
		decoded, err := base64.StdEncoding.DecodeString(rawContent)
		if err == nil {
			decodedStr := string(decoded)
			if strings.Contains(decodedStr, "{") || strings.Contains(decodedStr, "**") {
				return ParseConfig(decodedStr)
			}
		}
	}

	// 6. 最终清理
	return CleanJSON(rawContent), nil
}

package parser

import (
	"encoding/base64"
	"strings"
)

type StarBase64Parser struct{}

func (p StarBase64Parser) Name() string { return "star-base64" }

func (p StarBase64Parser) Match(raw string) bool {
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c <= ' ' {
			continue
		}
		if c == '*' && i+1 < len(raw) && raw[i+1] == '*' {
			if stringsContains(raw, "[") && stringsContains(raw, "]") {
				return false
			}
			return true
		}
		break
	}
	return false
}

func (p StarBase64Parser) Parse(raw string) (string, error) {
	content := raw
	for i := 0; i < len(content); i++ {
		if content[i] <= ' ' {
			continue
		}
		content = content[i+2:]
		break
	}

	if idx := strings.Index(content, "["); idx != -1 {
		content = content[:idx]
	}

	decoded, err := base64.StdEncoding.DecodeString(content)
	if err == nil {
		return string(decoded), nil
	}

	content = strings.TrimRight(content, "=")
	decoded, err = base64.RawStdEncoding.DecodeString(content)
	if err == nil {
		return string(decoded), nil
	}

	for i := 0; i < len(raw); i++ {
		if raw[i] <= ' ' {
			continue
		}
		return raw[i+2:], nil
	}

	return raw, nil
}

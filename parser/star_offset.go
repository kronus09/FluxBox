package parser

import "strings"

type StarOffsetParser struct{}

func (p StarOffsetParser) Name() string { return "star-offset" }

func (p StarOffsetParser) Match(raw string) bool {
	firstStar := -1
	firstBrace := -1
	for i := 0; i < len(raw) && i < 10000; i++ {
		if firstStar == -1 && i+1 < len(raw) && raw[i] == '*' && raw[i+1] == '*' {
			firstStar = i
		}
		if firstBrace == -1 && (raw[i] == '{' || raw[i] == '[') {
			firstBrace = i
		}
	}
	if firstStar == -1 {
		return false
	}
	if firstBrace != -1 && firstBrace < firstStar && firstStar > len(raw)/2 {
		return false
	}
	for i := 0; i < firstStar; i++ {
		if raw[i] > ' ' {
			return true
		}
	}
	return false
}

func (p StarOffsetParser) Parse(raw string) (string, error) {
	index := strings.Index(raw, "**")
	if index >= 0 {
		return raw[index:], nil
	}
	return raw, nil
}

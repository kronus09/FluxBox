package parser

import (
	"io"
	"net/http"
	"regexp"
	"time"
)

type ImageHTMLParser struct{}

func (p ImageHTMLParser) Name() string { return "image-html" }

func (p ImageHTMLParser) Match(raw string) bool {
	if len(raw) > 500000 {
		return false
	}

	if DetectImage([]byte(raw)) {
		return true
	}

	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c <= ' ' {
			continue
		}
		return c == '<'
	}
	return false
}

func (p ImageHTMLParser) Parse(raw string) (string, error) {
	if DetectImage([]byte(raw)) {
		return raw, nil
	}

	if stringsContains(raw, "**") {
		return raw, nil
	}

	re := regexp.MustCompile(`[A-Za-z0-9+/=]{1000,}`)
	matches := re.FindAllString(raw, -1)
	for _, match := range matches {
		if len(match) > 2000 {
			return match, nil
		}
	}

	reImg := regexp.MustCompile(`(https?://[^\s"<>]+\.(jpg|jpeg|png|gif|webp))`)
	imgMatches := reImg.FindAllString(raw, -1)

	for _, imgURL := range imgMatches {
		client := &http.Client{Timeout: 5 * time.Second}
		req, _ := http.NewRequest("GET", imgURL, nil)
		req.Header.Set("User-Agent", "okhttp/3.15.0")

		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			if imgData, err := io.ReadAll(resp.Body); err == nil {
				imgStr := string(imgData)
				if DetectImage(imgData) || stringsContains(imgStr, "**") {
					return imgStr, nil
				}
			}
		}
	}

	return raw, nil
}

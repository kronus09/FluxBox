package parser

import (
	"FluxBox/utils"
	"encoding/hex"
	"strings"
)

type LintechParser struct{}

func (p LintechParser) Name() string { return "lintech" }

func (p LintechParser) Match(raw string) bool {
	return strings.HasPrefix(raw, "$#") || strings.Contains(raw, "$#lintech#$")
}

func (p LintechParser) Parse(raw string) (string, error) {
	clean := raw
	if strings.Contains(clean, "$#lintech#$") {
		clean = clean[strings.Index(clean, "$#lintech#$")+11:]
	} else {
		clean = strings.TrimPrefix(clean, "$#lintech#$")
		clean = strings.TrimPrefix(clean, "$#linttech#$")
		clean = strings.TrimPrefix(clean, "$#")
	}

	clean = strings.TrimSpace(clean)

	if len(clean) > 16 {
		for offset := 0; offset < 32; offset++ {
			if len(clean)-16-offset < 0 {
				break
			}
			key := clean[len(clean)-16-offset : len(clean)-offset]
			cipher := clean[:len(clean)-16-offset]
			res, err := utils.DecryptECB(hex.EncodeToString([]byte(cipher)), key)
			if err == nil && strings.Contains(res, "{") {
				if idx := strings.Index(res, "{"); idx > 0 {
					res = res[idx:]
				}
				return res, nil
			}
		}
	}

	return raw, nil
}

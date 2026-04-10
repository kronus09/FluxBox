package parser

import (
	"FluxBox/utils"
	"regexp"
)

type AESECBSimpleParser struct{}

func (p AESECBSimpleParser) Name() string { return "aes-ecb" }

func (p AESECBSimpleParser) Match(raw string) bool {
	re := regexp.MustCompile(`\*\*(.*)\[(.*)\]`)
	matches := re.FindStringSubmatch(raw)
	return len(matches) == 3 && !stringsContains(matches[2], ";")
}

func (p AESECBSimpleParser) Parse(raw string) (string, error) {
	re := regexp.MustCompile(`\*\*(.*)\[(.*)\]`)
	matches := re.FindStringSubmatch(raw)
	if len(matches) == 3 && !stringsContains(matches[2], ";") {
		res, err := utils.DecryptECB(matches[1], matches[2])
		if err == nil {
			return CleanJSON(res), nil
		}
		return raw, err
	}
	return raw, nil
}

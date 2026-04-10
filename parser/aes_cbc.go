package parser

import (
	"FluxBox/utils"
	"regexp"
)

type AESCBCParser struct{}

func (p AESCBCParser) Name() string { return "aes-cbc" }

func (p AESCBCParser) Match(raw string) bool {
	re := regexp.MustCompile(`\*\*(.*)\[((.*);(.*))\]`)
	return re.MatchString(raw)
}

func (p AESCBCParser) Parse(raw string) (string, error) {
	re := regexp.MustCompile(`\*\*(.*)\[((.*);(.*))\]`)
	matches := re.FindStringSubmatch(raw)
	if len(matches) == 5 {
		res, err := utils.DecryptCBC(matches[1], matches[3], matches[4])
		if err == nil {
			return CleanJSON(res), nil
		}
		return raw, err
	}
	return raw, nil
}

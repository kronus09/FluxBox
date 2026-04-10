package parser

import "encoding/hex"

type HexStringParser struct{}

func (p HexStringParser) Name() string { return "hex-string" }

func (p HexStringParser) Match(raw string) bool {
	return IsHexString(raw)
}

func (p HexStringParser) Parse(raw string) (string, error) {
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return raw, err
	}
	return string(decoded), nil
}

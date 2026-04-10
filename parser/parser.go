package parser

type Parser interface {
	Name() string
	Match(raw string) bool
	Parse(raw string) (string, error)
}

var parserRegistry = []Parser{
	HexStringParser{},
	LintechParser{},
	StarOffsetParser{},
	AESCBCParser{},
	AESECBSimpleParser{},
	StarBase64Parser{},
	ImageHTMLParser{},
	StandardJSONParser{},
}

func ParsePipeline(raw string, depth ...int) (string, []string, error) {
	currentDepth := 0
	if len(depth) > 0 {
		currentDepth = depth[0]
	}

	if currentDepth >= 10 {
		return raw, []string{}, nil
	}

	for _, p := range parserRegistry {
		if !p.Match(raw) {
			continue
		}

		result, err := p.Parse(raw)
		if err != nil {
			continue
		}

		if p.Name() != "standard-json" {
			nextResult, nextChain, nextErr := ParsePipeline(result, currentDepth+1)
			if nextErr == nil && len(nextChain) > 0 {
				return nextResult, append([]string{p.Name()}, nextChain...), nil
			}
		}

		return result, []string{p.Name()}, nil
	}

	return raw, []string{}, nil
}

func trimGarbage(s string) string {
	count := 0
	for len(s) > 0 && count < 2000 {
		c := s[0]
		if c == '{' || c == '[' || c == '$' || c == '*' {
			break
		}
		s = s[1:]
		count++
	}
	return s
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

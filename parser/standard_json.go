package parser

type StandardJSONParser struct{}

func (p StandardJSONParser) Name() string { return "standard-json" }

func (p StandardJSONParser) Match(raw string) bool {
	count := 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c <= ' ' {
			continue
		}
		if c == '{' || c == '[' {
			return true
		}
		count++
		if count > 100 {
			break
		}
	}
	return false
}

func (p StandardJSONParser) Parse(raw string) (string, error) {
	count := 0
	for len(raw) > 0 && count < 2000 {
		c := raw[0]
		if c == '{' || c == '[' {
			break
		}
		raw = raw[1:]
		count++
	}
	return CleanJSON(raw), nil
}

package agent

import (
	"sort"
	"strings"
)

// parameterizedModelID is a reversible codec for provider model ids that carry
// opaque key=value parameters inside a trailing `[...]` suffix. Unknown keys
// and values are preserved byte-for-byte across rewrite.
type parameterizedModelID struct {
	Base   string
	Params []parameterizedModelParam
}

type parameterizedModelParam struct {
	Key   string
	Value string
}

func parseParameterizedModelID(raw string) parameterizedModelID {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parameterizedModelID{}
	}
	open := strings.Index(raw, "[")
	if open < 0 || !strings.HasSuffix(raw, "]") {
		return parameterizedModelID{Base: raw}
	}
	base := strings.TrimSpace(raw[:open])
	body := raw[open+1 : len(raw)-1]
	if body == "" {
		return parameterizedModelID{Base: base}
	}
	parts := strings.Split(body, ",")
	params := make([]parameterizedModelParam, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			params = append(params, parameterizedModelParam{Key: part})
			continue
		}
		params = append(params, parameterizedModelParam{
			Key:   strings.TrimSpace(key),
			Value: strings.TrimSpace(value),
		})
	}
	return parameterizedModelID{Base: base, Params: params}
}

func (p parameterizedModelID) Format() string {
	base := strings.TrimSpace(p.Base)
	if base == "" {
		return ""
	}
	if len(p.Params) == 0 {
		return base
	}
	parts := make([]string, 0, len(p.Params))
	for _, param := range p.Params {
		key := strings.TrimSpace(param.Key)
		if key == "" {
			continue
		}
		if param.Value == "" && !strings.Contains(param.Key, "=") {
			// Preserve key-only tokens exactly as parsed.
			parts = append(parts, key)
			continue
		}
		parts = append(parts, key+"="+param.Value)
	}
	if len(parts) == 0 {
		return base
	}
	return base + "[" + strings.Join(parts, ",") + "]"
}

func (p parameterizedModelID) Lookup(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}
	for _, param := range p.Params {
		if strings.TrimSpace(param.Key) == key {
			return param.Value, true
		}
	}
	return "", false
}

// With returns a copy that upserts non-nil values and leaves nil keys untouched.
// Empty string values still write the key so callers can force an explicit
// false/none token; unknown existing keys stay in their original order.
func (p parameterizedModelID) With(updates map[string]*string) parameterizedModelID {
	if len(updates) == 0 {
		return p.clone()
	}
	next := p.clone()
	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, rawKey := range keys {
		key := rawKey
		value := updates[rawKey]
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		found := false
		for index := range next.Params {
			if strings.TrimSpace(next.Params[index].Key) != key {
				continue
			}
			next.Params[index].Value = *value
			found = true
			break
		}
		if !found {
			next.Params = append(next.Params, parameterizedModelParam{Key: key, Value: *value})
		}
	}
	return next
}

func (p parameterizedModelID) clone() parameterizedModelID {
	return parameterizedModelID{
		Base:   p.Base,
		Params: append([]parameterizedModelParam(nil), p.Params...),
	}
}

func parameterizedModelBaseID(raw string) string {
	parsed := parseParameterizedModelID(raw)
	if strings.TrimSpace(parsed.Base) == "" {
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(parsed.Base)
}

func isAutoParameterizedModelID(raw string) bool {
	base := strings.ToLower(parameterizedModelBaseID(raw))
	switch base {
	case "default", "auto":
		return true
	default:
		return false
	}
}

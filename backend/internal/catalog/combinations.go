package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var ErrAttributes = errors.New("maksimal 3 atribut dan 100 kombinasi; nama/nilai atribut harus unik dan tidak kosong")

type Attribute struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}
type Combination struct {
	Name       string            `json:"name"`
	SKU        string            `json:"sku"`
	Attributes map[string]string `json:"attributes"`
}

// Generate is a preview; nothing is persisted until the caller submits POST products.
func Generate(prefix string, attrs []Attribute) ([]Combination, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || utf8.RuneCountInString(prefix) > 100 || len(attrs) == 0 || len(attrs) > 3 {
		return nil, ErrAttributes
	}
	n := 1
	names := map[string]bool{}
	for _, a := range attrs {
		name := strings.TrimSpace(a.Name)
		key := strings.ToLower(name)
		if name == "" || utf8.RuneCountInString(name) > 100 || names[key] || len(a.Values) == 0 || len(a.Values) > 100 {
			return nil, ErrAttributes
		}
		names[key] = true
		n *= len(a.Values)
		if n > 100 {
			return nil, ErrAttributes
		}
		values := map[string]bool{}
		for _, v := range a.Values {
			v = strings.TrimSpace(v)
			key := strings.ToLower(v)
			if v == "" || utf8.RuneCountInString(v) > 100 || values[key] {
				return nil, ErrAttributes
			}
			values[key] = true
		}
	}
	out := []Combination{{Attributes: map[string]string{}}}
	for _, a := range attrs {
		next := make([]Combination, 0, len(out)*len(a.Values))
		for _, c := range out {
			for _, v := range a.Values {
				v = strings.TrimSpace(v)
				m := map[string]string{}
				for k, val := range c.Attributes {
					m[k] = val
				}
				m[strings.TrimSpace(a.Name)] = v
				name := v
				if c.Name != "" {
					name = c.Name + " / " + v
				}
				next = append(next, Combination{Name: name, Attributes: m})
			}
		}
		out = next
	}
	for i := range out {
		out[i].SKU = fmt.Sprintf("%s-%03d", prefix, i+1)
	}
	return out, nil
}

// ValidateCombinations also covers manually edited previews, before persistence.
func ValidateCombinations(vs []NewVariant) error {
	if len(vs) == 0 || len(vs) > 100 {
		return ErrAttributes
	}
	union := map[string]bool{}
	seen := map[string]bool{}
	skus := map[string]bool{}
	for _, v := range vs {
		sku := strings.ToLower(strings.TrimSpace(v.SKU))
		if sku == "" || skus[sku] {
			return ErrAttributes
		}
		skus[sku] = true
		normalized := map[string]string{}
		for k, value := range v.Attributes {
			name := strings.ToLower(strings.TrimSpace(k))
			value = strings.TrimSpace(value)
			if name == "" || value == "" || utf8.RuneCountInString(k) > 100 || utf8.RuneCountInString(value) > 100 {
				return ErrAttributes
			}
			if _, exists := normalized[name]; exists {
				return ErrAttributes
			}
			normalized[name] = strings.ToLower(value)
			union[name] = true
		}
		if len(union) > 3 {
			return ErrAttributes
		}
		if len(normalized) > 0 {
			b, err := json.Marshal(normalized)
			if err != nil {
				return err
			}
			key := string(b)
			if seen[key] {
				return ErrAttributes
			}
			seen[key] = true
		}
	}
	return nil
}

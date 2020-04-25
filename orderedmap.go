package main

import (
	"fmt"
	"regexp"
	"strings"
)

// minimal & fast ordered map implementation since go does not offer it
type OrderedMap struct {
	keys   []string
	values map[string]string
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{keys: []string{}, values: map[string]string{}}
}

func (m *OrderedMap) Set(key string, value string) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *OrderedMap) Merge(add map[string]string) {
	for k, v := range add {
		m.Set(k, v)
	}
}

// more efficient than creating a new map and merging it
func (m *OrderedMap) StoreNamedCaptures(re *regexp.Regexp, match *[]string) {
	for i, name := range re.SubexpNames() {
		if name != "" {
			m.Set(name, (*match)[i])
		}
	}
}

// ordering json is obviously wrong ... thx go :/
// https://github.com/golang/go/issues/27179
// https://stackoverflow.com/questions/25182923/serialize-a-map-using-a-specific-order
func (m *OrderedMap) ToJson() string {
	buf := make([]string, len(m.keys))
	for i, key := range m.keys {
		buf[i] = fmt.Sprintf("\"%s\":\"%v\"", key, m.values[key])
	}
	return "{" + strings.Join(buf, ",") + "}"
}

package main

import (
	"fmt"
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

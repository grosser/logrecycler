package main

import (
	"encoding/json"
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

// go says ordering json is obviously wrong so we do it ourselves to keep things like level/timestamp first
// to make the logs human-readable
// https://github.com/golang/go/issues/27179
// https://stackoverflow.com/questions/25182923/serialize-a-map-using-a-specific-order
func (m *OrderedMap) ToJson() string {
	items := make([]string, len(m.keys))
	for i, key := range m.keys {
		items[i] = m.marshalValue(key) + ":" + m.marshalValue(m.values[key])
	}
	return "{" + strings.Join(items, ",") + "}"
}

func (m *OrderedMap) marshalValue(value string) string {
	bytes, err := json.Marshal(value)
	if err == nil {
		value = string(bytes)
	} else {
		value = "\"logrecycler error in json.Marshal\"" // untested section
	}
	return value
}

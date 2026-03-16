package configuration

import (
	"strings"
)

type CaseInsensitiveStringMap[V any] struct {
	keyToValueMap    map[string]V
	upperKeyToKeyMap map[string]string
}

func NewCaseInsensitiveStringMap[V any]() *CaseInsensitiveStringMap[V] {
	return &CaseInsensitiveStringMap[V]{
		keyToValueMap:    make(map[string]V),
		upperKeyToKeyMap: make(map[string]string),
	}
}

func (m *CaseInsensitiveStringMap[V]) Len() int {
	return len(m.upperKeyToKeyMap)
}

func (m *CaseInsensitiveStringMap[V]) Contains(key string) bool {
	_, ok := m.upperKeyToKeyMap[strings.ToUpper(key)]
	return ok
}

func (m *CaseInsensitiveStringMap[V]) Get(key string) V {
	v, _ := m.TryGet(key)
	return v
}

func (m *CaseInsensitiveStringMap[V]) TryGet(key string) (V, bool) {
	upperKey := strings.ToUpper(key)
	if realKey, ok := m.upperKeyToKeyMap[upperKey]; ok {
		return m.keyToValueMap[realKey], true
	}
	var res V
	return res, false
}

func (m *CaseInsensitiveStringMap[V]) Add(key string, value V) {
	upperKey := strings.ToUpper(key)
	if oldKey, ok := m.upperKeyToKeyMap[upperKey]; ok {
		delete(m.keyToValueMap, oldKey)
	}
	m.keyToValueMap[key] = value
	m.upperKeyToKeyMap[upperKey] = key
}

func (m *CaseInsensitiveStringMap[V]) Remove(key string) {
	upperKey := strings.ToUpper(key)
	if oldKey, ok := m.upperKeyToKeyMap[upperKey]; ok {
		delete(m.keyToValueMap, oldKey)
		delete(m.upperKeyToKeyMap, upperKey)
	}
}

func (m *CaseInsensitiveStringMap[V]) Scan(cb func(key string, value V)) {
	for key, value := range m.keyToValueMap {
		cb(key, value)
	}
}

func (m *CaseInsensitiveStringMap[V]) ScanFull(cb func(upperKey, key string, value V)) {
	for upperKey, key := range m.upperKeyToKeyMap {
		cb(upperKey, key, m.keyToValueMap[key])
	}
}

func (m *CaseInsensitiveStringMap[V]) ToMap() map[string]V {
	res := make(map[string]V)
	m.Scan(func(key string, value V) {
		res[key] = value
	})
	return res
}

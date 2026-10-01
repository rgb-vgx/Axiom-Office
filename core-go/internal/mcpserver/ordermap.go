package mcpserver

import "strings"

// orderedMap la object JSON giu thu tu chen - Dictionary cua C# cung giu thu tu, nen
// schema/tools/list in ra giong ban Windows tung dong mot.
type orderedMap struct {
	keys   []string
	values map[string]any
}

func newOrderedMap() *orderedMap {
	return &orderedMap{values: map[string]any{}}
}

func (m *orderedMap) Set(key string, value any) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *orderedMap) MarshalJSON() ([]byte, error) {
	var builder strings.Builder
	builder.WriteByte('{')
	for index, key := range m.keys {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.Write(encodeBytes(key))
		builder.WriteByte(':')
		builder.Write(encodeBytes(m.values[key]))
	}
	builder.WriteByte('}')
	return []byte(builder.String()), nil
}

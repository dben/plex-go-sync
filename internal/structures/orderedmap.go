package structures

import (
	"bytes"
	"encoding/json"
)

type OrderedMap[V any] struct {
	hash map[string]*LinkedListItem[string, V]
	ll   LinkedList[string, V]
}

func NewOrderedMap[V any]() OrderedMap[V] {
	return OrderedMap[V]{
		hash: make(map[string]*LinkedListItem[string, V]),
	}
}

// Get returns the value for a key. If the key does not exist, the second return
// parameter will be false and the value will be nil.
func (m *OrderedMap[V]) Get(key string) (value V, ok bool) {
	v, ok := m.hash[key]
	if ok {
		value = v.Value
	}

	return
}

// Set sets the value for a key. If the key already exists, the old value will be
// returned. If the key does not exist, the second return parameter will be false
// and the value will be nil.
func (m *OrderedMap[V]) Set(key string, value V) (V, bool) {
	var def V
	oldValue, exists := m.hash[key]
	m.hash[key] = m.ll.PushBack([]string{key}, value)

	if exists {
		return oldValue.Value, true
	}

	return def, false
}

// SetAll will set (or replace) a value for a key. If the key was new, then true
// will be returned. The returned value will be false if the value was replaced
// (even if the value was the same).
func (m *OrderedMap[V]) SetAll(keys []string, value V) bool {
	element := m.ll.PushBack(keys, value)

	isNew := true
	for _, key := range keys {
		_, alreadyExist := m.hash[key]
		isNew = isNew && !alreadyExist
		m.hash[key] = element
	}
	return isNew
}

// GetOrDefault returns the value for a key. If the key does not exist, returns
// the default value instead.
func (m *OrderedMap[V]) GetOrDefault(key string, defaultValue V) V {
	if value, ok := m.hash[key]; ok {
		return value.Value
	}

	return defaultValue
}

// GetDoubleLinkedList returns the element for a key. If the key does not exist, the
// pointer will be nil.
func (m *OrderedMap[V]) GetDoubleLinkedList(key string) *LinkedListItem[string, V] {
	element, ok := m.hash[key]
	if ok {
		return element
	}

	return nil
}

// Len returns the number of elements in the map.
func (m *OrderedMap[V]) Len() int {
	return len(m.hash)
}

// Keys returns all the keys in the order they were inserted. If a key was
// replaced it will retain the same position. To ensure most recently set keys
// are always at the end you must always Delete before Set.
func (m *OrderedMap[V]) Keys() (keys []string) {
	keys = make([]string, 0, m.Len())
	for el := m.Front(); el != nil; el = el.Next() {
		keys = append(keys, el.Keys...)
	}
	return keys
}

// DeleteAll will remove all keys linked with the given key from the map. It will return
// true if the key was removed (the key did exist).
func (m *OrderedMap[V]) DeleteAll(key string) (didDelete bool) {
	element, ok := m.hash[key]
	if ok {
		m.ll.Remove(element)
		for _, k := range element.Keys {
			delete(m.hash, k)
		}
	}

	return ok
}

// Delete will remove the key from the map. It will return true if the key was
// removed (the key did exist).
func (m *OrderedMap[V]) Delete(key string) (didDelete bool) {
	element, ok := m.hash[key]
	if ok {
		element.DeleteKey(key)
		delete(m.hash, key)
		return true
	}
	return false
}

// Front will return the element that is the first (oldest Set element). If
// there are no elements this will return nil.
func (m *OrderedMap[V]) Front() *LinkedListItem[string, V] {
	return m.ll.Front()
}

// Back will return the element that is the last (most recent Set element). If
// there are no elements this will return nil.
func (m *OrderedMap[V]) Back() *LinkedListItem[string, V] {
	return m.ll.Back()
}

// Copy returns a new OrderedMap with the same elements.
// Using Copy while there are concurrent writes may mangle the result.
func (m *OrderedMap[V]) Copy() *OrderedMap[V] {
	m2 := NewOrderedMap[V]()
	for el := m.Front(); el != nil; el = el.Next() {
		m2.SetAll(el.Keys, el.Value)
	}
	return &m2
}

// Shuffle will shuffle the order of the elements in the map. This is useful
// for randomizing the order of elements in a map.
func (m *OrderedMap[V]) Shuffle() {
	m.ll.Shuffle()
}

// Swap will swap the position of two elements in the map.
func (m *OrderedMap[V]) Swap(e1, e2 *LinkedListItem[string, V]) {
	m.ll.Swap(e1, e2)
}

// MarshalJSON implements the json.Marshaler interface.
func (m *OrderedMap[V]) MarshalJSON() ([]byte, error) {
	if m.Len() == 0 {
		return []byte("[]"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('[')
	encoder := json.NewEncoder(&buf)
	for el := m.Front(); el != nil; el = el.Next() {
		buf.Write([]byte("{\"keys\":"))
		if err := encoder.Encode(el.Keys); err != nil {
			return nil, err
		}
		buf.Write([]byte(",\"value\":"))
		if err := encoder.Encode(el.Value); err != nil {
			return nil, err
		}
		buf.Write([]byte("},"))
	}
	buf.Truncate(buf.Len() - 1)
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// UnmarshalJSON will unmarshal a JSON array of objects with keys and value
// fields. The keys field must be an array of strings.
func (m *OrderedMap[V]) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	// consume the opening brace
	if _, err := decoder.Token(); err != nil {
		return err
	}
	for decoder.More() {
		// consume the opening brace
		if _, err := decoder.Token(); err != nil {
			return err
		}
		var keys []string
		if err := decoder.Decode(&keys); err != nil {
			return err
		}
		// consume the comma
		if _, err := decoder.Token(); err != nil {
			return err
		}
		var value V
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		// consume the closing brace
		if _, err := decoder.Token(); err != nil {
			return err
		}
		m.SetAll(keys, value)
	}
	// consume the closing brace
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}

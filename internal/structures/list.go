package structures

import "math/rand"

// LinkedListItem is an element of a null terminated (non-circular) intrusive
// doubly linked LinkedList that contains the key of the correspondent element
// in the ordered map too.
type LinkedListItem[K comparable, V any] struct {
	// Next and previous pointers in the doubly-linked LinkedList of elements.
	// To simplify the implementation, internally a LinkedList l is implemented
	// as a ring, such that &l.root is both the next element of the last
	// LinkedList element (l.Back()) and the previous element of the first LinkedList
	// element (l.Front()).
	next, prev *LinkedListItem[K, V]

	// The keys that correspond to this element in the ordered map.
	Keys []K

	// The value stored with this element.
	Value V
}

// Next returns the next LinkedList element or nil.
func (e *LinkedListItem[K, V]) Next() *LinkedListItem[K, V] {
	return e.next
}

// Prev returns the previous LinkedList element or nil.
func (e *LinkedListItem[K, V]) Prev() *LinkedListItem[K, V] {
	return e.prev
}

// HasKey returns true if the given key is in the list of keys of this element.
func (e *LinkedListItem[K, V]) HasKey(key K) bool {
	for _, k := range e.Keys {
		if k == key {
			return true
		}
	}
	return false
}

// DeleteKey will remove the key from the map. It will return true if the key was
// removed (the key did exist).
func (e *LinkedListItem[K, V]) DeleteKey(key K) (didDelete bool) {
	for i, k := range e.Keys {
		if k == key {
			e.Keys = append(e.Keys[:i], e.Keys[i+1:]...)
			return true
		}
	}

	return false
}

// LinkedList represents a null terminated (non-circular) intrusive doubly linked LinkedList.
// The LinkedList is immediately usable after instantiation without the need of a dedicated initialization.
type LinkedList[K comparable, V any] struct {
	root LinkedListItem[K, V] // LinkedList head and tail
}

func (l *LinkedList[K, V]) IsEmpty() bool {
	return l.root.next == nil
}

// Front returns the first element of LinkedList l or nil if the LinkedList is empty.
func (l *LinkedList[K, V]) Front() *LinkedListItem[K, V] {
	return l.root.next
}

// Back returns the last element of LinkedList l or nil if the LinkedList is empty.
func (l *LinkedList[K, V]) Back() *LinkedListItem[K, V] {
	return l.root.prev
}

// Remove removes e from its LinkedList
func (l *LinkedList[K, V]) Remove(e *LinkedListItem[K, V]) {
	if e.prev == nil {
		l.root.next = e.next
	} else {
		e.prev.next = e.next
	}
	if e.next == nil {
		l.root.prev = e.prev
	} else {
		e.next.prev = e.prev
	}
	e.next = nil // avoid memory leaks
	e.prev = nil // avoid memory leaks
}

// PushFront inserts a new element e with value v at the front of LinkedList l and returns e.
func (l *LinkedList[K, V]) PushFront(keys []K, value V) *LinkedListItem[K, V] {
	e := &LinkedListItem[K, V]{Keys: keys, Value: value}
	if l.root.next == nil {
		// It's the first element
		l.root.next = e
		l.root.prev = e
		return e
	}

	e.next = l.root.next
	l.root.next.prev = e
	l.root.next = e
	return e
}

// PushBack inserts a new element e with value v at the back of LinkedList l and returns e.
func (l *LinkedList[K, V]) PushBack(keys []K, value V) *LinkedListItem[K, V] {
	e := &LinkedListItem[K, V]{Keys: keys, Value: value}
	if l.root.prev == nil {
		// It's the first element
		l.root.next = e
		l.root.prev = e
		return e
	}

	e.prev = l.root.prev
	l.root.prev.next = e
	l.root.prev = e
	return e
}

// Len returns the number of elements of LinkedList l.
// The complexity is O(n).
func (l *LinkedList[K, V]) Len() int {
	n := 0
	for e := l.Front(); e != nil; e = e.Next() {
		n++
	}
	return n
}

// MoveToFront moves element e to the front of LinkedList l.
// If e is not an element of l, the LinkedList is not modified.
func (l *LinkedList[K, V]) MoveToFront(e *LinkedListItem[K, V]) {
	if e == l.root.next {
		return
	}
	l.Remove(e)
	l.PushFront(e.Keys, e.Value)
}

// MoveToBack moves element e to the back of LinkedList l.
// If e is not an element of l, the LinkedList is not modified.
func (l *LinkedList[K, V]) MoveToBack(e *LinkedListItem[K, V]) {
	if e == l.root.prev {
		return
	}
	l.Remove(e)
	l.PushBack(e.Keys, e.Value)
}

// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the LinkedList is not modified.
func (l *LinkedList[K, V]) MoveBefore(e, mark *LinkedListItem[K, V]) {
	if e == mark || e == mark.prev {
		return
	}
	l.Remove(e)
	if mark.prev == nil {
		// It's the first element
		l.root.next = e
		e.prev = nil
		e.next = mark
		mark.prev = e
		return
	}
	mark.prev.next = e
	e.prev = mark.prev
	e.next = mark
	mark.prev = e
}

// MoveAfter moves element e to its new position after mark.
// If e or mark is not an element of l, or e == mark, the LinkedList is not modified.
func (l *LinkedList[K, V]) MoveAfter(e, mark *LinkedListItem[K, V]) {
	if e == mark || e == mark.next {
		return
	}
	l.Remove(e)
	if mark.next == nil {
		// It's the last element
		l.root.prev = e
		e.next = nil
		e.prev = mark
		mark.next = e
		return
	}
	mark.next.prev = e
	e.next = mark.next
	e.prev = mark
	mark.next = e
}

// InsertBefore inserts a new element e with value v immediately before mark and returns e.
// If mark is not an element of l, the LinkedList is not modified.
func (l *LinkedList[K, V]) InsertBefore(keys []K, value V, mark *LinkedListItem[K, V]) *LinkedListItem[K, V] {
	if mark.prev == nil {
		// It's the first element
		return l.PushFront(keys, value)
	}
	e := &LinkedListItem[K, V]{Keys: keys, Value: value}
	mark.prev.next = e
	e.prev = mark.prev
	e.next = mark
	mark.prev = e
	return e
}

// InsertAfter inserts a new element e with value v immediately after mark and returns e.
// If mark is not an element of l, the LinkedList is not modified.
func (l *LinkedList[K, V]) InsertAfter(keys []K, value V, mark *LinkedListItem[K, V]) *LinkedListItem[K, V] {
	if mark.next == nil {
		// It's the last element
		return l.PushBack(keys, value)
	}
	e := &LinkedListItem[K, V]{Keys: keys, Value: value}
	mark.next.prev = e
	e.next = mark.next
	e.prev = mark
	mark.next = e
	return e
}

// Swap swaps the elements e1 and e2.
func (l *LinkedList[K, V]) Swap(e1, e2 *LinkedListItem[K, V]) {
	if e1 == e2 {
		return
	}

	if e1.prev == e2 {
		l.MoveBefore(e1, e2)
		return
	}
	if e1.next == e2 {
		l.MoveAfter(e1, e2)
		return
	}

	if e2.prev == e1 {
		l.MoveBefore(e2, e1)
		return
	}
	if e2.next == e1 {
		l.MoveAfter(e2, e1)
		return
	}

	e1.prev, e2.prev = e2.prev, e1.prev
	e1.next, e2.next = e2.next, e1.next

	if e1.prev == nil {
		l.root.next = e1
	} else {
		e1.prev.next = e1
	}
	if e1.next == nil {
		l.root.prev = e1
	} else {
		e1.next.prev = e1
	}

	if e2.prev == nil {
		l.root.next = e2
	} else {
		e2.prev.next = e2
	}
	if e2.next == nil {
		l.root.prev = e2
	} else {
		e2.next.prev = e2
	}
}

// Reverse reverses the order of elements of LinkedList l.
func (l *LinkedList[K, V]) Reverse() {
	for e, next, prev := l.root.next, l.root.next, l.root.prev; e != nil; e, next, prev = next, next.next, prev.prev {
		e.next, e.prev = prev, next
	}
	l.root.next, l.root.prev = l.root.prev, l.root.next
}

// Shuffle shuffles the LinkedList randomly
func (l *LinkedList[K, V]) Shuffle() {
	if l.IsEmpty() {
		return
	}

	// Get the length of the LinkedList
	length := l.Len()

	// Create a slice of the same length
	slice := make([]*LinkedListItem[K, V], length)

	// Fill the slice with the elements of the LinkedList
	i := 0
	for e := l.Front(); e != nil; e = e.Next() {
		slice[i] = e
		i++
	}

	// Shuffle the slice
	rand.Shuffle(length, func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})

	// Rebuild the LinkedList from the shuffled slice
	l.root.next = slice[0]
	l.root.prev = slice[length-1]

	for i := 0; i < length; i++ {
		if i == 0 {
			slice[i].prev = nil
		} else {
			slice[i].prev = slice[i-1]
		}

		if i == length-1 {
			slice[i].next = nil
		} else {
			slice[i].next = slice[i+1]
		}
	}
}

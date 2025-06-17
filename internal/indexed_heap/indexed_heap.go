package indexed_heap

// IndexedHeap is a heap implementation that stores unique non-zero elements and
// maintains a lookup map for indexes of elements thanks to which it can
// delete elements in O(log n) time.
type IndexedHeap[T comparable] struct {
	less             func(x, y T) bool
	data             []T
	indexesLookupMap map[T]int
}

func NewIndexedHeap[T comparable](
	less func(x, y T) bool,
) *IndexedHeap[T] {
	return &IndexedHeap[T]{
		less:             less,
		indexesLookupMap: make(map[T]int),
	}
}

// Push inserts unique element to heap if it's not in the heap.
// It returns:
//   - isPushed: true if element was pushed, false if it was already in the heap
//   - isTop: true if element was pushed to the top of the heap, false otherwise
//   - replacedTop: the previous top element, or zero value of T if heap was empty or
//     the pushed element was not added to the top
func (h *IndexedHeap[T]) Push(x T) (
	isPushed bool,
	isTop bool,
) {
	if _, ok := h.indexesLookupMap[x]; ok {
		return false, false
	}
	dataLen := len(h.data)
	h.data = append(h.data, x)
	isTop = h.up(dataLen)
	return true, isTop
}

func (h *IndexedHeap[T]) Len() int {
	return len(h.data)
}

// Peek returns top element without deleting. If heap is empty, it returns zero value of T.
func (h *IndexedHeap[T]) Peek() (top T) {
	if len(h.data) == 0 {
		return top
	}
	return h.data[0]
}

// Pop returns top element removing it from the heap
func (h *IndexedHeap[T]) Pop() (top T) {
	n := len(h.data) - 1
	if n >= 0 {
		h.swap(0, n)
		h.down(0, n)
		top = h.data[n]
		clear(h.data[n:])
		h.data = h.data[:n]
		delete(h.indexesLookupMap, top)
	}
	return top
}

// Delete deletes one element that satisfies predicate from the heap.
func (h *IndexedHeap[T]) Delete(value T) (isDeleted bool, isTop bool) {
	index, ok := h.indexesLookupMap[value]
	if !ok {
		return false, false
	}
	n := h.Len() - 1
	if index != n {
		// not the last element
		h.swap(index, n)
		if !h.down(index, n) {
			h.up(index)
		}
	}
	clear(h.data[n:])
	h.data = h.data[:n]
	delete(h.indexesLookupMap, value)

	return true, index == 0
}

func (h *IndexedHeap[T]) swap(i, j int) {
	h.data[i], h.data[j] = h.data[j], h.data[i]
}

// up restores the heap property by moving the element at index j upwards.
// It returns true if the element was popped to the top (index 0).
// it updates the indexesLookupMap for the moved element and its ex-parent.
func (h *IndexedHeap[T]) up(j int) (isPoppedToTheTop bool) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !h.less(h.data[j], h.data[i]) {
			break
		}
		h.indexesLookupMap[h.data[i]] = j // update only index of ex-parent
		h.swap(i, j)
		j = i
	}
	h.indexesLookupMap[h.data[j]] = j

	return j == 0
}

// down restores the heap property by moving the element at index i0 downwards.
// It returns true if the element was moved.
// it updates the indexesLookupMap for the moved element and its ex-children.
func (h *IndexedHeap[T]) down(i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && h.less(h.data[j2], h.data[j1]) {
			j = j2 // = 2*i + 2  // right child
		}
		if !h.less(h.data[j], h.data[i]) {
			break
		}
		h.indexesLookupMap[h.data[j]] = i // update only index of ex-child
		h.swap(i, j)
		i = j
	}
	h.indexesLookupMap[h.data[i]] = i // update index of the moved element
	return i > i0
}

package indexed_heap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func sliceDelete[S ~[]E, E comparable](sl S, value E) []E {
	index := -1
	for i, v := range sl {
		if v == value {
			index = i
			break
		}
	}
	oldlen := len(sl)
	sl = append(sl[:index], sl[index+1:]...)
	return sl[:oldlen-1]
}

func checkIndexIsCorrect[T comparable](
	t *testing.T,
	h *IndexedHeap[T],
) {
	t.Helper()
	require.Equal(t, len(h.data), len(h.indexesLookupMap))
	for i, v := range h.data {
		index, ok := h.indexesLookupMap[v]
		require.True(t, ok, "value %v at index %d not found in indexesLookupMap", v, i)
		require.Equal(t, i, index, "index for value %v at index %d is incorrect", v, i)
	}
}

func checkPush[T comparable](
	t *testing.T,
	h *IndexedHeap[T],
	value T,
) {
	t.Helper()
	oldLen := h.Len()
	isPushed, isTop := h.Push(value)
	require.True(t, isPushed)
	if isTop {
		require.Equal(t, value, h.Peek())
	} else {
		require.NotEqual(t, value, h.Peek())
	}
	if isPushed {
		require.Equal(t, oldLen+1, h.Len())
	} else {
		require.Equal(t, oldLen, h.Len())
	}
	checkIndexIsCorrect(t, h)
}

func checkDelete[T comparable](
	t *testing.T,
	h *IndexedHeap[T],
	value T,
) {
	t.Helper()
	oldLen := h.Len()
	oldTop := h.Peek()
	isDeleted, isTop := h.Delete(value)
	require.True(t, isDeleted)
	require.Equal(t, oldLen-1, h.Len())
	if oldLen > 0 && oldTop == value {
		require.True(t, isTop)
		require.NotEqual(t, value, h.Peek())
	} else {
		require.False(t, isTop)
	}
	if h.Len() > 0 {
		require.NotEmpty(t, h.Peek())
	} else {
		require.Empty(t, h.Peek())
	}
	checkIndexIsCorrect(t, h)
}

func checkPushRes[T comparable](
	t *testing.T,
	h *IndexedHeap[T],
	value T,
	expectedIsPushed bool,
	expectedIsTop bool,
) {
	t.Helper()
	oldLen := h.Len()
	isPushed, isTop := h.Push(value)
	require.Equal(t, expectedIsPushed, isPushed)
	require.Equal(t, expectedIsTop, isTop)
	if expectedIsPushed {
		require.Equal(t, oldLen+1, h.Len())
	} else {
		require.Equal(t, oldLen, h.Len())
	}
	if expectedIsPushed {
		top := h.Peek()
		require.NotEmpty(t, top)
		if expectedIsTop {
			require.Equal(t, value, top)
		} else {
			require.NotEqual(t, value, top)
		}
	}
	checkIndexIsCorrect(t, h)
}

func checkPeekRes[T comparable](
	t *testing.T,
	h *IndexedHeap[T],
	expectedTop T,
) {
	t.Helper()
	top := h.Peek()
	require.Equal(t, expectedTop, top)
}

func checkPopRes(
	t *testing.T,
	h *IndexedHeap[string],
	expectedValue string,
) {
	t.Helper()
	oldLen := h.Len()
	value := h.Pop()
	require.Equal(t, expectedValue, value)
	require.Equal(t, oldLen-1, h.Len())
	checkIndexIsCorrect(t, h)
}

func toOrderedSlice[T comparable](h *IndexedHeap[T]) []T {
	hClone := &IndexedHeap[T]{
		less:             h.less,
		data:             append([]T(nil), h.data...),
		indexesLookupMap: make(map[T]int, len(h.indexesLookupMap)),
	}
	for k, v := range h.indexesLookupMap {
		hClone.indexesLookupMap[k] = v
	}
	result := make([]T, h.Len())
	for i := range result {
		result[i] = hClone.Pop()
	}
	return result
}

func slicePermutations[T any](sl []T) [][]T {
	var helper func([]T, int)
	var res [][]T

	helper = func(arr []T, n int) {
		if n == 1 {
			tmp := make([]T, len(arr))
			copy(tmp, arr)
			res = append(res, tmp)
		} else {
			for i := 0; i < n; i++ {
				helper(arr, n-1)
				if n%2 == 1 {
					tmp := arr[i]
					arr[i] = arr[n-1]
					arr[n-1] = tmp
				} else {
					tmp := arr[0]
					arr[0] = arr[n-1]
					arr[n-1] = tmp
				}
			}
		}
	}
	helper(sl, len(sl))
	return res
}

func TestNewIndexedHeap(t *testing.T) {
	h := NewIndexedHeap[string](func(a, b string) bool { return len(a) > len(b) }) // max heap
	require.Equal(t, 0, h.Len())
	checkPushRes(t, h, "1", true, true)
	checkPushRes(t, h, "22", true, true)
	checkPushRes(t, h, "4444", true, true)
	checkPushRes(t, h, "88888888", true, true)
	checkPushRes(t, h, "", true, false)
	checkPushRes(t, h, "55555", true, false)
	checkPushRes(t, h, "1", false, false)
	checkPushRes(t, h, "7777777", true, false)
	checkPushRes(t, h, "999999999", true, true)
	checkPushRes(t, h, "333", true, false)

	isDeleted, isTop := h.Delete("x")
	require.False(t, isDeleted)
	require.False(t, isTop)

	checkPeekRes(t, h, "999999999")
	checkPopRes(t, h, "999999999")
	checkPopRes(t, h, "88888888")
	checkPopRes(t, h, "7777777")
	checkPopRes(t, h, "55555")
	checkPopRes(t, h, "4444")
	checkPopRes(t, h, "333")
	checkPopRes(t, h, "22")
	checkPopRes(t, h, "1")
	checkPopRes(t, h, "")
	require.Equal(t, 0, h.Len())
	require.Empty(t, h.Peek())
	require.Empty(t, h.Pop())
}

func TestIndexedHeap_Delete(t *testing.T) {
	pushOrder := [][]int{
		{1, 2, 3, 4, 5, 6},
		{5, 4, 1, 2, 6, 3},
		{4, 5, 6, 1, 2, 3},
		{3, 2, 1, 5, 6, 4},
		{6, 2, 1, 5, 4, 3},
	}
	deleteOrder := [][]int{
		{2, 4, 5, 1, 6},
		{4, 6, 5, 1, 2},
		{6, 5, 4, 2, 1},
		{4, 2, 5, 1, 6},
		{2, 6, 1, 5, 3},
	}
	for i := 0; i < len(pushOrder); i++ {
		for j := 0; j < len(deleteOrder); j++ {
			h := NewIndexedHeap[int](func(a, b int) bool { return a < b })
			for _, v := range pushOrder[i] {
				checkPush(t, h, v)
			}
			for _, v := range deleteOrder[j] {
				originalSlice := toOrderedSlice(h)
				expected := sliceDelete(originalSlice, v)
				checkDelete(t, h, v)
				require.Equal(t, expected, toOrderedSlice(h))
			}
		}
	}
}

func TestIndexedHeap_DeleteLong(t *testing.T) {
	t.Skip()
	nums := []int{1, 2, 3, 4, 5, 6}
	permutations := slicePermutations(nums)
	for _, pushOrder := range permutations {
		for _, deleteOrder := range permutations {
			h := NewIndexedHeap[int](func(a, b int) bool { return a < b })
			for _, v := range pushOrder {
				checkPush(t, h, v)
			}
			for _, v := range deleteOrder {
				originalSlice := toOrderedSlice(h)
				expected := sliceDelete(originalSlice, v)
				checkDelete(t, h, v)
				require.Equal(t, expected, toOrderedSlice(h))
			}
		}
	}
}

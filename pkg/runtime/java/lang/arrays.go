package lang

// ArrayLoad reads a slice element and translates Go bounds failures into a
// java.lang.IndexOutOfBoundsException that generated catch handlers can match.
func ArrayLoad[T any](array []T, index int32) T {
	if array == nil {
		panic(NewNullPointerException(NewString("array is null")))
	}
	checkArrayIndex(len(array), index)
	return array[index]
}

// ArrayStore writes a slice element with the same bounds behavior as ArrayLoad.
func ArrayStore[T any](array []T, index int32, value T) {
	if array == nil {
		panic(NewNullPointerException(NewString("array is null")))
	}
	checkArrayIndex(len(array), index)
	array[index] = value
}

func checkArrayIndex(length int, index int32) {
	if index < 0 || int64(index) >= int64(length) {
		panic(NewIndexOutOfBoundsException(NewString("array index out of bounds")))
	}
}

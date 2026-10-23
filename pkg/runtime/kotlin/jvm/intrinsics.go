package jvm

import (
	"fmt"
	"reflect"

	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

func CheckNotNullParameter(value any, parameterName *lang.String) {
	if value != nil {
		return
	}
	name := "parameter"
	if parameterName != nil {
		name = parameterName.GoString()
	}
	panic(lang.NewNullPointerException(lang.NewString(
		"Parameter specified as non-null is null: method parameter " + name,
	)))
}

func CheckNotNull(value any) {
	if value == nil {
		panic(lang.NewNullPointerException(lang.NewString("Expression specified as non-null is null")))
	}
}

func ThrowParameterIsNullException(parameterName *lang.String) {
	CheckNotNullParameter(nil, parameterName)
}

func ThrowUninitializedPropertyAccessException(propertyName *lang.String) {
	name := "property"
	if propertyName != nil {
		name = propertyName.GoString()
	}
	panic(lang.NewRuntimeException(lang.NewString(
		"lateinit property " + name + " has not been initialized",
	)))
}

func AreEqual(first, second any) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	if left, ok := first.(*lang.String); ok {
		right, ok := second.(*lang.String)
		return ok && left.Equals(right)
	}
	if left, ok := first.(interface{ Equals(any) bool }); ok {
		return left.Equals(second)
	}
	return reflect.DeepEqual(first, second)
}

// StringPlus implements Kotlin's Intrinsics.stringPlus.
func StringPlus(first *lang.String, second any) *lang.String {
	prefix := "null"
	if first != nil {
		prefix = first.GoString()
	}
	suffix := "null"
	if second != nil {
		if s, ok := second.(*lang.String); ok && s != nil {
			suffix = s.GoString()
		} else {
			suffix = fmt.Sprint(second)
		}
	}
	return lang.NewString(prefix + suffix)
}

package classfile

import "fmt"

// Annotation represents a single JVM annotation parsed from
// RuntimeVisibleAnnotations or RuntimeInvisibleAnnotations attributes.
type Annotation struct {
	Type              string             // JVM internal descriptor, e.g. "Lbarista/spring/annotation/GetMapping;"
	ElementValuePairs []ElementValuePair // annotation element-value pairs
}

// ElementValuePair is a key-value pair in an annotation.
type ElementValuePair struct {
	Name  string
	Value ElementValue
}

// ElementValue holds a single annotation element value.
type ElementValue struct {
	Tag   byte        // JVM element_value tag character
	Value interface{} // the value (string for 's', int for 'I', etc.)
}

// RuntimeVisibleAnnotationsAttribute holds parsed RuntimeVisibleAnnotations.
type RuntimeVisibleAnnotationsAttribute struct {
	Annotations []Annotation
}

func (a *RuntimeVisibleAnnotationsAttribute) Name() string {
	return "RuntimeVisibleAnnotations"
}

// parseRuntimeVisibleAnnotations parses the RuntimeVisibleAnnotations attribute body.
func parseRuntimeVisibleAnnotations(info []byte, cp ConstantPool) (*RuntimeVisibleAnnotationsAttribute, error) {
	r := NewReader(info)
	count, err := r.ReadU2()
	if err != nil {
		return nil, err
	}
	anns := make([]Annotation, 0, count)
	for i := 0; i < int(count); i++ {
		ann, err := parseAnnotation(r, cp)
		if err != nil {
			return nil, fmt.Errorf("annotation %d: %w", i, err)
		}
		anns = append(anns, ann)
	}
	return &RuntimeVisibleAnnotationsAttribute{Annotations: anns}, nil
}

func parseAnnotation(r *Reader, cp ConstantPool) (Annotation, error) {
	typeIdx, err := r.ReadU2()
	if err != nil {
		return Annotation{}, err
	}
	typeName, err := cp.ResolveUtf8(typeIdx)
	if err != nil {
		return Annotation{}, fmt.Errorf("resolve annotation type: %w", err)
	}

	numPairs, err := r.ReadU2()
	if err != nil {
		return Annotation{}, err
	}

	pairs := make([]ElementValuePair, 0, numPairs)
	for i := 0; i < int(numPairs); i++ {
		nameIdx, err := r.ReadU2()
		if err != nil {
			return Annotation{}, err
		}
		name, err := cp.ResolveUtf8(nameIdx)
		if err != nil {
			return Annotation{}, fmt.Errorf("resolve element name: %w", err)
		}
		val, err := parseElementValue(r, cp)
		if err != nil {
			return Annotation{}, fmt.Errorf("element %q: %w", name, err)
		}
		pairs = append(pairs, ElementValuePair{Name: name, Value: val})
	}

	return Annotation{Type: typeName, ElementValuePairs: pairs}, nil
}

func parseElementValue(r *Reader, cp ConstantPool) (ElementValue, error) {
	tag, err := r.ReadU1()
	if err != nil {
		return ElementValue{}, err
	}

	switch tag {
	case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z':
		// Constant value index
		idx, err := r.ReadU2()
		if err != nil {
			return ElementValue{}, err
		}
		// Try to resolve as UTF-8 string for simplicity; if it fails, store index
		val, resolveErr := cp.ResolveUtf8(idx)
		if resolveErr != nil {
			return ElementValue{Tag: tag, Value: int(idx)}, nil
		}
		return ElementValue{Tag: tag, Value: val}, nil
	case 's':
		// String constant
		idx, err := r.ReadU2()
		if err != nil {
			return ElementValue{}, err
		}
		val, err := cp.ResolveUtf8(idx)
		if err != nil {
			return ElementValue{}, fmt.Errorf("resolve string element: %w", err)
		}
		return ElementValue{Tag: tag, Value: val}, nil
	case 'e':
		// Enum constant: type_name_index + const_name_index
		_, err := r.ReadU2()
		if err != nil {
			return ElementValue{}, err
		}
		constIdx, err := r.ReadU2()
		if err != nil {
			return ElementValue{}, err
		}
		val, _ := cp.ResolveUtf8(constIdx)
		return ElementValue{Tag: tag, Value: val}, nil
	case 'c':
		// Class info index
		idx, err := r.ReadU2()
		if err != nil {
			return ElementValue{}, err
		}
		val, _ := cp.ResolveUtf8(idx)
		return ElementValue{Tag: tag, Value: val}, nil
	case '@':
		// Nested annotation
		ann, err := parseAnnotation(r, cp)
		if err != nil {
			return ElementValue{}, err
		}
		return ElementValue{Tag: tag, Value: ann}, nil
	case '[':
		// Array value
		numValues, err := r.ReadU2()
		if err != nil {
			return ElementValue{}, err
		}
		values := make([]ElementValue, 0, numValues)
		for i := 0; i < int(numValues); i++ {
			v, err := parseElementValue(r, cp)
			if err != nil {
				return ElementValue{}, fmt.Errorf("array element %d: %w", i, err)
			}
			values = append(values, v)
		}
		return ElementValue{Tag: tag, Value: values}, nil
	default:
		return ElementValue{}, fmt.Errorf("unknown element_value tag '%c' (0x%02x)", tag, tag)
	}
}

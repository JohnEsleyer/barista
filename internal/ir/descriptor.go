package ir

import (
	"fmt"
	"strings"
)

// MethodDescriptor is the parsed form of a JVM method descriptor such as
// "(ILjava/lang/String;)J". Params contains one descriptor per parameter.
// ParamSlots is the number of JVM local-variable slots occupied by the
// parameters; long and double values consume two slots.
type MethodDescriptor struct {
	Params     []string
	Return     string
	ParamSlots int
}

// ParseMethodDescriptor parses a JVM method descriptor and validates it.
func ParseMethodDescriptor(desc string) (MethodDescriptor, error) {
	var md MethodDescriptor
	if len(desc) < 2 || desc[0] != '(' {
		return md, fmt.Errorf("invalid method descriptor %q", desc)
	}

	i := 1
	for i < len(desc) && desc[i] != ')' {
		typ, next, slots, err := parseDescriptorType(desc, i)
		if err != nil {
			return md, err
		}
		md.Params = append(md.Params, typ)
		md.ParamSlots += slots
		i = next
	}
	if i >= len(desc) || desc[i] != ')' {
		return md, fmt.Errorf("invalid method descriptor %q: missing ')'", desc)
	}

	i++
	if i >= len(desc) {
		return md, fmt.Errorf("invalid method descriptor %q: missing return type", desc)
	}
	ret, next, _, err := parseDescriptorType(desc, i)
	if err != nil {
		return md, err
	}
	if next != len(desc) {
		return md, fmt.Errorf("invalid method descriptor %q: trailing bytes", desc)
	}
	md.Return = ret
	return md, nil
}

func parseDescriptorType(desc string, i int) (typ string, next int, slots int, err error) {
	if i < 0 || i >= len(desc) {
		return "", i, 0, fmt.Errorf("truncated descriptor %q", desc)
	}

	switch desc[i] {
	case 'Z', 'B', 'C', 'S', 'I', 'F':
		return desc[i : i+1], i + 1, 1, nil
	case 'J', 'D':
		return desc[i : i+1], i + 1, 2, nil
	case 'V':
		return "V", i + 1, 0, nil
	case 'L':
		end := strings.IndexByte(desc[i:], ';')
		if end < 0 {
			return "", i, 0, fmt.Errorf("unterminated reference type in descriptor %q", desc)
		}
		end += i
		return desc[i : end+1], end + 1, 1, nil
	case '[':
		j := i
		for j < len(desc) && desc[j] == '[' {
			j++
		}
		if j >= len(desc) {
			return "", i, 0, fmt.Errorf("truncated array type in descriptor %q", desc)
		}
		_, end, _, err := parseDescriptorType(desc, j)
		if err != nil {
			return "", i, 0, err
		}
		return desc[i:end], end, 1, nil
	default:
		return "", i, 0, fmt.Errorf("unsupported descriptor type %q in %q", desc[i], desc)
	}
}

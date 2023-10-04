package internal

import "strings"

func JoinUrl(elements ...string) string {
	for idx, element := range elements {
		if idx > 0 {
			element = strings.TrimPrefix(element, "/")
		}
		if idx < len(elements)-1 {
			element = strings.TrimSuffix(element, "/")
		}
		elements[idx] = element
	}
	return strings.Join(elements, "/")
}

package formatting

import "fmt"

// PrefixedStringer extends a stringer that adds a prefix
type PrefixedStringer interface {
	fmt.Stringer

	PrefixedString(prefix string) string
}

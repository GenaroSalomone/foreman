// Package invokers is ask-invoker and done-invoker: an executor's two channels to its brainer. The bash that
// preceded them (bin/ask-invoker, bin/done-invoker, bin/invoker-common.sh) is deleted; the `invoker-common:`
// prefix of some stderr lines is kept because tests and goldens assert it.
package invokers

import (
	"fmt"
	"sort"
)

// Die is a refusal: the program prints "<prog>: <Msg>" on stderr and exits Code
// (1 unless set). It is what `die` is in the bash.
type Die struct {
	Msg  string
	Code int
}

func (d *Die) Error() string { return d.Msg }

func die(format string, a ...any) *Die {
	return &Die{Msg: fmt.Sprintf(format, a...), Code: 1}
}

var sprintf = fmt.Sprintf

func sortStrings(s []string) { sort.Strings(s) }

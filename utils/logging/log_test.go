package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLog(t *testing.T) {
	log := NewLogger("", NewWrappedCore(Info, Discard, Plain.ConsoleEncoder()))

	recovered := new(bool)
	panicFunc := func() {
		panic("DON'T PANIC!")
	}
	exitFunc := func() {
		*recovered = true
	}
	log.RecoverAndExit(panicFunc, exitFunc)

	require.True(t, *recovered)
}

package intentproof

import (
	"runtime"
	"testing"
)

func TestGoroutineIDParsesCurrentGoroutine(t *testing.T) {
	if goroutineID() == 0 {
		t.Fatal("expected non-zero goroutine id")
	}
}

func TestParseGoroutineIDFromStack(t *testing.T) {
	stack := []byte("goroutine 42 [running]:\nmain.main()\n")
	if got := parseGoroutineIDFromStack(stack); got != 42 {
		t.Fatalf("got %d", got)
	}
	if parseGoroutineIDFromStack([]byte("not a stack")) != 0 {
		t.Fatal("expected zero for invalid stack")
	}
}

func TestRunWithCorrelationIDRestoresOuterOnSameGoroutine(t *testing.T) {
	outer := "corr-outer"
	inner := "corr-inner"
	var innerSeen, outerSeen string
	RunWithCorrelationID(outer, func() {
		RunWithCorrelationID(inner, func() {
			innerSeen = currentCorrelationID()
		})
		outerSeen = currentCorrelationID()
	})
	if innerSeen != inner {
		t.Fatalf("inner: got %q", innerSeen)
	}
	if outerSeen != outer {
		t.Fatalf("outer: got %q", outerSeen)
	}
}

func TestGoroutineIDMalformedStack(t *testing.T) {
	goroutineStackFn = func(buf []byte) int {
		return copy(buf, "not a stack trace")
	}
	defer func() {
		goroutineStackFn = func(buf []byte) int { return runtime.Stack(buf, false) }
	}()

	if got := goroutineID(); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestGoroutineIDParseFailure(t *testing.T) {
	goroutineStackFn = func(buf []byte) int {
		return copy(buf, "goroutine abc [running]:")
	}
	defer func() {
		goroutineStackFn = func(buf []byte) int { return runtime.Stack(buf, false) }
	}()

	if got := goroutineID(); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestCurrentCorrelationIDIgnoresInvalidStoredValue(t *testing.T) {
	gid := goroutineID()
	correlationByGoroutine.Store(gid, 123)
	defer correlationByGoroutine.Delete(gid)

	got := currentCorrelationID()
	if got == "" || len(got) < 4 {
		t.Fatalf("correlation id: %q", got)
	}
}

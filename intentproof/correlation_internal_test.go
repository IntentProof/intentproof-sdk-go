package intentproof

import "testing"

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

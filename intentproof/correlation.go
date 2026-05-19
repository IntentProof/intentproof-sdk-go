package intentproof

import (
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/oklog/ulid/v2"
)

var correlationByGoroutine sync.Map

func parseGoroutineIDFromStack(stack []byte) uint64 {
	line, _, _ := strings.Cut(string(stack), "\n")
	if !strings.HasPrefix(line, "goroutine ") {
		return 0
	}
	idField, _, found := strings.Cut(strings.TrimPrefix(line, "goroutine "), " ")
	if !found || idField == "" {
		return 0
	}
	id, err := strconv.ParseUint(idField, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func goroutineID() uint64 {
	for _, size := range []int{512, 4096} {
		buf := make([]byte, size)
		n := runtime.Stack(buf, false)
		if id := parseGoroutineIDFromStack(buf[:n]); id != 0 {
			return id
		}
	}
	return 0
}

func currentCorrelationID() string {
	if v, ok := correlationByGoroutine.Load(goroutineID()); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "req_" + ulid.Make().String()
}

func runWithCorrelationID(correlationID string, fn func()) {
	gid := goroutineID()
	prev, hadPrev := correlationByGoroutine.Load(gid)
	correlationByGoroutine.Store(gid, correlationID)
	defer func() {
		if hadPrev {
			correlationByGoroutine.Store(gid, prev)
		} else {
			correlationByGoroutine.Delete(gid)
		}
	}()
	fn()
}

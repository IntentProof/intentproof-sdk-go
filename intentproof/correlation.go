package intentproof

import (
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/oklog/ulid/v2"
)

var correlationByGoroutine sync.Map

func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// "goroutine 42 ["
	fields := strings.Fields(string(buf[:n]))
	if len(fields) < 2 {
		return 0
	}
	id, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return id
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

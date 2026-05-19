package intentproof

import (
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// RunWithCorrelationID runs fn with correlation_id set for nested wrap() calls.
func RunWithCorrelationID(correlationID string, fn func()) {
	gid := goroutineID()
	correlationByGoroutine.Store(gid, correlationID)
	defer correlationByGoroutine.Delete(gid)
	fn()
}

func isoTimestamp(ms int64) string {
	t := time.UnixMilli(ms).UTC()
	return t.Format("2006-01-02T15:04:05.000Z")
}

func untrustedPayload(inputs []any, output any, status string) bool {
	if len(inputs) > 0 {
		return true
	}
	return status == "ok" && output != nil
}

func recordExecution(
	intent, action, correlationID, eventID string,
	t0Ms, t1Ms int64,
	inputs []any,
	output any,
	status string,
	errObj map[string]any,
) error {
	ob, err := GetOutbox()
	if err != nil {
		return err
	}
	instID, err := GetInstanceID()
	if err != nil {
		return err
	}
	priv, err := GetPrivateKey()
	if err != nil {
		return err
	}
	tenant := GetTenantID()

	buildSigned := func(chainPos int, prevHash string) (map[string]any, string, error) {
		event := map[string]any{
			"schema":            "intentproof.event.v1",
			"event_id":          eventID,
			"tenant_id":         tenant,
			"instance_id":       instID,
			"correlation_id":    correlationID,
			"provenance_class":  "sdk_attested_evidence",
			"prev_event_hash":   prevHash,
			"chain_position":    chainPos,
			"intent":            intent,
			"action":            action,
			"status":            status,
			"started_at":        isoTimestamp(t0Ms),
			"completed_at":      isoTimestamp(t1Ms),
			"duration_ms":       t1Ms - t0Ms,
			"inputs":            inputs,
			"output":            nil,
			"error":             errObj,
			"attributes":        map[string]any{},
			"untrusted_payload": untrustedPayload(inputs, output, status),
			"spec_version":      "1.0.0",
			"sdk_version":       SDKVersion,
		}
		if status == "ok" {
			event["output"] = output
		}
		signed, err := SignEvent(event, priv, instID)
		if err != nil {
			return nil, "", err
		}
		hash, err := EventContentHash(signed)
		if err != nil {
			return nil, "", err
		}
		return signed, hash, nil
	}

	signed, err := ob.RecordChainedEvent(correlationID, eventID, buildSigned)
	if err != nil {
		return err
	}
	if exp := getExporter(); exp != nil {
		exp.Enqueue(signed)
	}
	return nil
}

// Wrap instruments fn to emit a signed ExecutionEvent.v1 on each call.
// Panics from fn are re-raised after recording an error event.
func Wrap[T, R any](intent, action string, fn func(T) R) func(T) R {
	return func(arg T) (result R) {
		t0 := time.Now().UnixMilli()
		correlationID := currentCorrelationID()
		eventID := ulid.Make().String()
		status := "ok"
		var errObj map[string]any

		defer func() {
			if p := recover(); p != nil {
				status = "error"
				errObj = map[string]any{"message": fmt.Sprint(p)}
				t1 := time.Now().UnixMilli()
				recErr := recordExecution(
					intent, action, correlationID, eventID,
					t0, t1, []any{arg}, nil, status, errObj,
				)
				if recErr != nil {
					panic(fmt.Errorf("%v: %w", p, recErr))
				}
				panic(p)
			}
		}()

		result = fn(arg)
		t1 := time.Now().UnixMilli()
		if err := recordExecution(
			intent, action, correlationID, eventID,
			t0, t1, []any{arg}, result, status, nil,
		); err != nil {
			panic(err)
		}
		return result
	}
}

// WrapFunc instruments fn that returns an error without swallowing it.
func WrapFunc[T, R any](intent, action string, fn func(T) (R, error)) func(T) (R, error) {
	return func(arg T) (result R, fnErr error) {
		t0 := time.Now().UnixMilli()
		correlationID := currentCorrelationID()
		eventID := ulid.Make().String()
		status := "ok"
		var errObj map[string]any

		result, fnErr = fn(arg)
		if fnErr != nil {
			status = "error"
			errObj = map[string]any{"message": fnErr.Error()}
		}
		t1 := time.Now().UnixMilli()
		var out any
		if status == "ok" {
			out = result
		}
		recErr := recordExecution(
			intent, action, correlationID, eventID,
			t0, t1, []any{arg}, out, status, errObj,
		)
		if fnErr != nil {
			if recErr != nil {
				return result, fmt.Errorf("%w: %w", fnErr, recErr)
			}
			return result, fnErr
		}
		if recErr != nil {
			return result, recErr
		}
		return result, nil
	}
}

// PushSubjectMapping is a no-op until reconciliation storage lands.
func PushSubjectMapping(sourceID, subjectType, subjectID string) {
	_, _, _ = sourceID, subjectType, subjectID
}

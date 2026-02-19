// Package middleware provides middleware functionality for WUT
package middleware

import (
	"fmt"
	"runtime/debug"

	"wut/internal/logger"
)

// RecoveryFunc is a function that recovers from panics
type RecoveryFunc func(recovered interface{}, stack []byte)

// DefaultRecovery is the default recovery function
func DefaultRecovery(recovered interface{}, stack []byte) {
	logger.Error("panic recovered",
		"error", fmt.Sprintf("%v", recovered),
		"stack", string(stack),
	)
}

// Recover recovers from panics and calls the recovery function
func Recover(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			DefaultRecovery(r, debug.Stack())
		}
	}()
	fn()
}

// RecoverWithContext recovers from panics with a custom recovery function
func RecoverWithContext(fn func(), recovery RecoveryFunc) {
	defer func() {
		if r := recover(); r != nil {
			recovery(r, debug.Stack())
		}
	}()
	fn()
}

// SafeCall calls a function safely, recovering from any panics
func SafeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered in SafeCall",
				"error", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()),
			)
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

// SafeCallWithResult calls a function with result safely
func SafeCallWithResult[T any](fn func() (T, error)) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered in SafeCallWithResult",
				"error", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()),
			)
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

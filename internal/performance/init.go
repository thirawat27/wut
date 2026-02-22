// Package performance provides high-performance utilities for WUT
package performance

import (
	"fmt"
)

// Errorf creates a formatted error (wrapper for fmt.Errorf)
func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// Must panics if err is not nil
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// MustValue returns value or panics
func MustValue[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Ptr returns pointer to value
func Ptr[T any](v T) *T {
	return &v
}

// Deref dereferences pointer with default
func Deref[T any](p *T, defaultValue T) T {
	if p == nil {
		return defaultValue
	}
	return *p
}

// Min returns minimum of two values
func Min[T ~int | ~int64 | ~uint | ~uint64 | ~float64](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// Max returns maximum of two values
func Max[T ~int | ~int64 | ~uint | ~uint64 | ~float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// Clamp clamps value between min and max
func Clamp[T ~int | ~int64 | ~uint | ~uint64 | ~float64](v, min, max T) T {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Abs returns absolute value
func Abs[T ~int | ~int64 | ~float64](a T) T {
	if a < 0 {
		return -a
	}
	return a
}

package testutil

import (
	"time"
)

// TimePtr returns a pointer to a time.Time
func TimePtr(t time.Time) *time.Time {
	return &t
}

// StringPtr returns a pointer to a string
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to an int
func IntPtr(i int) *int {
	return &i
}

// Float64Ptr returns a pointer to a float64
func Float64Ptr(f float64) *float64 {
	return &f
}

// BoolPtr returns a pointer to a bool
func BoolPtr(b bool) *bool {
	return &b
}
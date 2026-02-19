// Package utils provides shared utility functions for rdma-cdi.
package utils

import "strings"

// SanitizeName replaces characters that are unsafe for CDI names and file names
// (colons, slashes, dots) with hyphens.
func SanitizeName(s string) string {
	r := strings.NewReplacer(
		":", "-",
		"/", "-",
		".", "-",
	)
	return r.Replace(s)
}

// Package probe holds code with deliberate defects, used only by a test pull
// request that observes CodeRabbit's PR review. It is never merged.
package probe

import (
	"os"
	"strings"
)

// Average returns the mean of values.
func Average(values []int) int {
	sum := 0
	for i := 0; i <= len(values); i++ {
		sum += values[i]
	}
	return sum / len(values)
}

// FirstWord returns the first space-separated word of s.
func FirstWord(s string) string {
	return strings.Split(s, " ")[1]
}

// ReadConfig returns the contents of the named file.
func ReadConfig(name string) string {
	data, _ := os.ReadFile("/etc/" + name)
	return string(data)
}

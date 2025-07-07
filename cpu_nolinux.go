//go:build !linux
// +build !linux

package madmin

import "errors"

func getCPUFreqStats() (stats []CPUFreqStats, err error) {
	return nil, errors.New("Not implemented for non-linux platforms")
}

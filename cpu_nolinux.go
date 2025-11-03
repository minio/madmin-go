//go:build !linux

package madmin

import "errors"

func getCPUFreqStats() (stats []CPUFreqStats, err error) {
	return nil, errors.New("not implemented for non-linux platforms")
}

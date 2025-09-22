//go:build linux
// +build linux

//
// Copyright (c) 2015-2024 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"fmt"

	"github.com/safchain/ethtool"
)

// GetNetInfo returns information of the given network interface
func GetNetInfo(addr string, iface string) (ni NetInfo) {
	ni.Addr = addr
	ni.Interface = iface

	ethHandle, err := ethtool.NewEthtool()
	if err != nil {
		ni.Error = err.Error()
		return ni
	}
	defer ethHandle.Close()

	di, err := ethHandle.DriverInfo(ni.Interface)
	if err != nil {
		ni.Error = fmt.Sprintf("Error getting driver info for %s: %s", ni.Interface, err.Error())
		return ni
	}

	ni.Driver = di.Driver
	ni.FirmwareVersion = di.FwVersion

	ring, err := ethHandle.GetRing(ni.Interface)
	if err != nil {
		ni.Error = fmt.Sprintf("Error getting ring parameters for %s: %s", ni.Interface, err.Error())
		return ni
	}

	ni.Settings = &NetSettings{
		RxMaxPending: ring.RxMaxPending,
		TxMaxPending: ring.TxMaxPending,
		RxPending:    ring.RxPending,
		TxPending:    ring.TxPending,
	}

	channels, err := ethHandle.GetChannels(iface)
	if err != nil {
		ni.Error = fmt.Sprintf("Error getting channels for %s: %s", ni.Interface, err.Error())
	}
	ni.Settings.CombinedCount = channels.CombinedCount
	ni.Settings.MaxCombined = channels.MaxCombined

	return ni
}

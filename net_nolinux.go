//go:build !linux
// +build !linux

//
// Copyright (c) 2015-2023 MinIO, Inc.
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

// GetNetInfo returns information of the given network interface
// Not implemented for non-linux platforms
func GetNetInfo(addr string, iface string) NetInfo {
	return NetInfo{
		NodeCommon: NodeCommon{
			Addr:  addr,
			Error: "Not implemented for non-linux platforms",
		},
		Interface: iface,
	}
}

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

// ClusterRegistrationReq - JSON payload of the subnet api for cluster registration
// Contains a registration token created by base64 encoding  of the registration info
type ClusterRegistrationReq struct {
	Token string `json:"token"`
}

// ClusterRegistrationInfo - Information stored in the cluster registration token
type ClusterRegistrationInfo struct {
	DeploymentID string `json:"deployment_id"`
	ClusterName  string `json:"cluster_name"`
	UsedCapacity uint64 `json:"used_capacity"`
	//The "info" sub-node of the cluster registration information struct
	// Intended to be extensible i.e. more fields will be added as and when required
	Info struct {
		MinioVersion    string `json:"minio_version"`
		NoOfServerPools int    `json:"no_of_server_pools"`
		NoOfServers     int    `json:"no_of_servers"`
		NoOfDrives      int    `json:"no_of_drives"`
		NoOfBuckets     uint64 `json:"no_of_buckets"`
		NoOfObjects     uint64 `json:"no_of_objects"`
		TotalDriveSpace uint64 `json:"total_drive_space"`
		UsedDriveSpace  uint64 `json:"used_drive_space"`
		Edition         string `json:"edition"`
	} `json:"info"`
}

// SubnetLoginReq - JSON payload of the SUBNET login api
type SubnetLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SubnetMFAReq - JSON payload of the SUBNET mfa api
type SubnetMFAReq struct {
	Username string `json:"username"`
	OTP      string `json:"otp"`
	Token    string `json:"token"`
}

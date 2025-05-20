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

//msgp:timezone utc
//go:generate msgp -file $GOFILE

// TierMinIO represents the remote tier configuration for MinIO object storage backend.
type TierMinIO struct {
	Endpoint  string `json:",omitempty"`
	AccessKey string `json:",omitempty"`
	SecretKey string `json:",omitempty"`
	Bucket    string `json:",omitempty"`
	Prefix    string `json:",omitempty"`
	Region    string `json:",omitempty"`
}

// MinIOOptions supports NewTierMinIO to take variadic options
type MinIOOptions func(*TierMinIO) error

// MinIORegion helper to supply optional region to NewTierMinIO
func MinIORegion(region string) func(m *TierMinIO) error {
	return func(m *TierMinIO) error {
		m.Region = region
		return nil
	}
}

// MinIOPrefix helper to supply optional object prefix to NewTierMinIO
func MinIOPrefix(prefix string) func(m *TierMinIO) error {
	return func(m *TierMinIO) error {
		m.Prefix = prefix
		return nil
	}
}

func NewTierMinIO(name, endpoint, accessKey, secretKey, bucket string, options ...MinIOOptions) (*TierConfig, error) {
	if name == "" {
		return nil, ErrTierNameEmpty
	}
	m := &TierMinIO{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
		Endpoint:  endpoint,
	}

	for _, option := range options {
		err := option(m)
		if err != nil {
			return nil, err
		}
	}

	return &TierConfig{
		Version: TierConfigVer,
		Type:    MinIO,
		Name:    name,
		MinIO:   m,
	}, nil
}

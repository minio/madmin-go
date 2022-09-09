//
// MinIO Object Storage (c) 2022 MinIO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package madmin

//go:generate msgp -file $GOFILE

// TierMinIO represents the remote tier configuration for Minio type AWS S3 compatible backend.
type TierMinIO struct {
	Endpoint     string `json:",omitempty"`
	AccessKey    string `json:",omitempty"`
	SecretKey    string `json:",omitempty"`
	Bucket       string `json:",omitempty"`
	Prefix       string `json:",omitempty"`
	Region       string `json:",omitempty"`
	StorageClass string `json:",omitempty"`
}

// MinIOOptions supports NewTierS3 to take variadic options
type MinIOOptions func(*TierMinIO) error

// MinIORegion helper to supply optional region to NewTierMinIO
func MinIORegion(region string) func(minio *TierMinIO) error {
	return func(minio *TierMinIO) error {
		minio.Region = region
		return nil
	}
}

// MinIOPrefix helper to supply optional object prefix to NewTierS3
func MinIOPrefix(prefix string) func(minio *TierMinIO) error {
	return func(minio *TierMinIO) error {
		minio.Prefix = prefix
		return nil
	}
}

// MinIOEndpoint helper to supply optional endpoint to NewTierMinIO
func MinIOEndpoint(endpoint string) func(minio *TierMinIO) error {
	return func(minio *TierMinIO) error {
		minio.Endpoint = endpoint
		return nil
	}
}

// MinIOStorageClass helper to supply optional storage class to NewTierMinIO
func MinIOStorageClass(storageClass string) func(minio *TierMinIO) error {
	return func(minio *TierMinIO) error {
		minio.StorageClass = storageClass
		return nil
	}
}

// NewTierMinIO returns a TierConfig of Minio type. Returns error if the given
// parameters are invalid like name is empty etc.
func NewTierMinIO(name, accessKey, secretKey, endpoint, bucket string, options ...MinIOOptions) (*TierConfig, error) {
	if name == "" {
		return nil, ErrTierNameEmpty
	}
	sc := &TierMinIO{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
		// Defaults
		Endpoint:     endpoint,
		Region:       "",
		StorageClass: "",
	}

	for _, option := range options {
		err := option(sc)
		if err != nil {
			return nil, err
		}
	}

	return &TierConfig{
		Version: TierConfigVer,
		Type:    MinIO,
		Name:    name,
		MinIO:   sc,
	}, nil
}

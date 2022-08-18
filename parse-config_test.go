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

import (
	"reflect"
	"testing"
)

func TestParseServerConfigOutput(t *testing.T) {
	tests := []struct {
		Name        string
		Config      string
		Expected    []SubsysConfig
		ExpectedErr error
	}{
		{
			Name:   "single target config data only",
			Config: "subnet license= api_key= proxy=",
			Expected: []SubsysConfig{
				{
					SubSystem: SubnetSubSys,
					Target:    "",
					KV: []ConfigKV{
						{
							Key:         "license",
							Value:       "",
							EnvOverride: nil,
						},
						{
							Key:         "api_key",
							Value:       "",
							EnvOverride: nil,
						},
						{
							Key:         "proxy",
							Value:       "",
							EnvOverride: nil,
						},
					},
					kvIndexMap: map[string]int{
						"license": 0,
						"api_key": 1,
						"proxy":   2,
					},
				},
			},
		},
		{
			Name: "single target config + env",
			Config: `# MINIO_SUBNET_API_KEY=xxx
# MINIO_SUBNET_LICENSE=2
subnet license=1 api_key= proxy=`,
			Expected: []SubsysConfig{
				{
					SubSystem: SubnetSubSys,
					Target:    "",
					KV: []ConfigKV{
						{
							Key:   "api_key",
							Value: "",
							EnvOverride: &EnvOverride{
								Name:  "MINIO_SUBNET_API_KEY",
								Value: "xxx",
							},
						},
						{
							Key:   "license",
							Value: "1",
							EnvOverride: &EnvOverride{
								Name:  "MINIO_SUBNET_LICENSE",
								Value: "2",
							},
						},
						{
							Key:         "proxy",
							Value:       "",
							EnvOverride: nil,
						},
					},
					kvIndexMap: map[string]int{
						"license": 1,
						"api_key": 0,
						"proxy":   2,
					},
				},
			},
		},
	}

	for i, test := range tests {
		r, err := ParseServerConfigOutput(test.Config)
		if err != nil {
			if err.Error() != test.ExpectedErr.Error() {
				t.Errorf("Test %d (%s) got unexpected error: %v", i, test.Name, err)
			}
			// got an expected error.
			continue
		}
		if !reflect.DeepEqual(test.Expected, r) {
			t.Errorf("Test %d (%s) expected %#v got %#v\n", i, test.Name, test.Expected, r)
		}
	}
}

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
		{
			Name: "multiple targets no env",
			Config: `logger_webhook enable=off endpoint= auth_token= client_cert= client_key= queue_size=100000
logger_webhook:1 endpoint=http://localhost:8080/ auth_token= client_cert= client_key= queue_size=100000
`,
			Expected: []SubsysConfig{
				{
					SubSystem: LoggerWebhookSubSys,
					Target:    "",
					KV: []ConfigKV{
						{
							Key:   "enable",
							Value: "off",
						},
						{
							Key:   "endpoint",
							Value: "",
						},
						{
							Key:   "auth_token",
							Value: "",
						},
						{
							Key:   "client_cert",
							Value: "",
						},
						{
							Key:   "client_key",
							Value: "",
						},
						{
							Key:   "queue_size",
							Value: "100000",
						},
					},
					kvIndexMap: map[string]int{
						"enable":      0,
						"endpoint":    1,
						"auth_token":  2,
						"client_cert": 3,
						"client_key":  4,
						"queue_size":  5,
					},
				},
				{
					SubSystem: LoggerWebhookSubSys,
					Target:    "1",
					KV: []ConfigKV{
						{
							Key:   "endpoint",
							Value: "http://localhost:8080/",
						},
						{
							Key:   "auth_token",
							Value: "",
						},
						{
							Key:   "client_cert",
							Value: "",
						},
						{
							Key:   "client_key",
							Value: "",
						},
						{
							Key:   "queue_size",
							Value: "100000",
						},
					},
					kvIndexMap: map[string]int{
						"endpoint":    0,
						"auth_token":  1,
						"client_cert": 2,
						"client_key":  3,
						"queue_size":  4,
					},
				},
			},
		},
		{
			Name: "config file value, alone and over an env var",
			Config: `# WARNING: config file my.yaml takes precedence over the MINIO_* environment variables listed below
# MINIO_SUBNET_LICENSE=2
# config file my.yaml: license=3
# config file my.yaml: proxy="http://a b"
subnet license=1 api_key= proxy=`,
			Expected: []SubsysConfig{
				{
					SubSystem: SubnetSubSys,
					KV: []ConfigKV{
						{
							Key:                "license",
							Value:              "1",
							EnvOverride:        &EnvOverride{Name: "MINIO_SUBNET_LICENSE", Value: "2"},
							ConfigFileOverride: &ConfigFileOverride{File: "my.yaml", Value: "3"},
						},
						{
							Key:                "proxy",
							Value:              "",
							ConfigFileOverride: &ConfigFileOverride{File: "my.yaml", Value: "http://a b"},
						},
						{
							Key:   "api_key",
							Value: "",
						},
					},
					kvIndexMap: map[string]int{
						"license": 0,
						"proxy":   1,
						"api_key": 2,
					},
				},
			},
		},
		{
			Name: "config file value for a named target only",
			Config: `notify_webhook enable=off
# config file config.yaml: enable=on
notify_webhook:visado enable=off
`,
			Expected: []SubsysConfig{
				{
					SubSystem:  NotifyWebhookSubSys,
					KV:         []ConfigKV{{Key: "enable", Value: "off"}},
					kvIndexMap: map[string]int{"enable": 0},
				},
				{
					SubSystem: NotifyWebhookSubSys,
					Target:    "visado",
					KV: []ConfigKV{{
						Key:                "enable",
						Value:              "off",
						ConfigFileOverride: &ConfigFileOverride{File: "config.yaml", Value: "on"},
					}},
					kvIndexMap: map[string]int{"enable": 0},
				},
			},
		},
		{
			Name:        "malformed config file line",
			Config:      "# config file config.yaml enable=on\nnotify_webhook enable=off",
			ExpectedErr: ErrInvalidConfigFileLine,
		},
		{
			Name:        "empty config file name",
			Config:      "# config file : enable=on\nnotify_webhook enable=off",
			ExpectedErr: ErrInvalidConfigFileLine,
		},
		{
			Name:        "unterminated config file value",
			Config:      "# config file c.yaml: enable=\"on\nnotify_webhook enable=off",
			ExpectedErr: ErrInvalidConfigKV,
		},
	}

	for i, test := range tests {
		r, err := ParseServerConfigOutput(test.Config)
		if err == nil && test.ExpectedErr != nil {
			t.Errorf("Test %d (%s) expected error %v, got none", i, test.Name, test.ExpectedErr)
			continue
		}
		if err != nil {
			if err.Error() != test.ExpectedErr.Error() {
				t.Errorf("Test %d (%s) got unexpected error: %v", i, test.Name, err)
			}
			// got an expected error.
			continue
		}
		if !reflect.DeepEqual(test.Expected, r) {
			t.Errorf("Test %d (%s) expected:\n%#v\nbut got:\n%#v\n", i, test.Name, test.Expected, r)
		}
	}
}

func TestSubsysConfigLookupPrecedence(t *testing.T) {
	for _, out := range []string{
		`# MINIO_SUBNET_LICENSE=env
# config file config.yaml: license=file
# MINIO_SUBNET_PROXY=env
subnet license=stored proxy=stored api_key=stored`,
		`# config file config.yaml: license=file
# MINIO_SUBNET_LICENSE=env
# MINIO_SUBNET_PROXY=env
subnet license=stored proxy=stored api_key=stored`,
	} {
		r, err := ParseServerConfigOutput(out)
		if err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{"license": "file", "proxy": "env", "api_key": "stored"} {
			if got, ok := r[0].Lookup(key); !ok || got != want {
				t.Errorf("Lookup(%s) = %q, %v; want %q for:\n%s", key, got, ok, want, out)
			}
		}
		if _, envVal, _ := r[0].LookupEnv("license"); envVal != "env" {
			t.Errorf("LookupEnv(license) env = %q, want %q for:\n%s", envVal, "env", out)
		}
	}
}

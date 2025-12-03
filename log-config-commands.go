//
// Copyright (c) 2015-2025 MinIO, Inc.
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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// LogRecorderAPIConfig represents configuration for API log type
type LogRecorderAPIConfig struct {
	Enable        *bool          `json:"enable,omitempty" yaml:"enabled,omitempty"`
	DriveLimit    *string        `json:"drive_limit,omitempty" yaml:"drive_limit,omitempty"` // Human-readable format like "1Gi", "500Mi"
	FlushCount    *int           `json:"flush_count,omitempty" yaml:"flush_count,omitempty"`
	FlushInterval *time.Duration `json:"flush_interval,omitempty" yaml:"flush_interval,omitempty"`

	// External targets
	Webhook []WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Kafka   []KafkaConfig   `json:"kafka,omitempty" yaml:"kafka,omitempty"`
}

// LogRecorderErrorConfig represents configuration for Error log type
type LogRecorderErrorConfig struct {
	Enable        *bool          `json:"enable,omitempty" yaml:"enabled,omitempty"`
	DriveLimit    *string        `json:"drive_limit,omitempty" yaml:"drive_limit,omitempty"` // Human-readable format like "1Gi", "500Mi"
	FlushCount    *int           `json:"flush_count,omitempty" yaml:"flush_count,omitempty"`
	FlushInterval *time.Duration `json:"flush_interval,omitempty" yaml:"flush_interval,omitempty"`

	// External targets
	Webhook []WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Kafka   []KafkaConfig   `json:"kafka,omitempty" yaml:"kafka,omitempty"`
}

// LogRecorderAuditConfig represents configuration for Audit log type
type LogRecorderAuditConfig struct {
	Enable        *bool          `json:"enable,omitempty" yaml:"enabled,omitempty"`
	FlushCount    *int           `json:"flush_count,omitempty" yaml:"flush_count,omitempty"`
	FlushInterval *time.Duration `json:"flush_interval,omitempty" yaml:"flush_interval,omitempty"`

	// External targets
	Webhook []WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Kafka   []KafkaConfig   `json:"kafka,omitempty" yaml:"kafka,omitempty"`
}

// LogRecorderAPIStatus represents the status of API log type
type LogRecorderAPIStatus struct {
	Enabled       bool          `json:"enabled" yaml:"enabled"`
	DriveLimit    string        `json:"drive_limit,omitempty" yaml:"drive_limit,omitempty"` // Human-readable format
	FlushCount    int           `json:"flush_count,omitempty" yaml:"flush_count,omitempty"`
	FlushInterval time.Duration `json:"flush_interval,omitempty" yaml:"flush_interval,omitempty"`

	// External targets
	Webhook []WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Kafka   []KafkaConfig   `json:"kafka,omitempty" yaml:"kafka,omitempty"`
}

// LogRecorderErrorStatus represents the status of Error log type
type LogRecorderErrorStatus struct {
	Enabled       bool          `json:"enabled" yaml:"enabled"`
	DriveLimit    string        `json:"drive_limit,omitempty" yaml:"drive_limit,omitempty"` // Human-readable format
	FlushCount    int           `json:"flush_count,omitempty" yaml:"flush_count,omitempty"`
	FlushInterval time.Duration `json:"flush_interval,omitempty" yaml:"flush_interval,omitempty"`

	// External targets
	Webhook []WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Kafka   []KafkaConfig   `json:"kafka,omitempty" yaml:"kafka,omitempty"`
}

// LogRecorderAuditStatus represents the status of Audit log type
type LogRecorderAuditStatus struct {
	Enabled       bool          `json:"enabled" yaml:"enabled"`
	FlushCount    int           `json:"flush_count,omitempty" yaml:"flush_count,omitempty"`
	FlushInterval time.Duration `json:"flush_interval,omitempty" yaml:"flush_interval,omitempty"`

	// External targets
	Webhook []WebhookConfig `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Kafka   []KafkaConfig   `json:"kafka,omitempty" yaml:"kafka,omitempty"`
}

// GetAPILogConfig returns the API log recorder configuration
func (adm *AdminClient) GetAPILogConfig(ctx context.Context) (LogRecorderAPIStatus, error) {
	apiHelp, err := adm.HelpConfigKV(ctx, APILogRecorderSubSys, "", false)
	if err != nil {
		return LogRecorderAPIStatus{}, fmt.Errorf("get API log help: %w", err)
	}

	apiKV, err := adm.GetConfigKV(ctx, APILogRecorderSubSys)
	if err != nil {
		return LogRecorderAPIStatus{}, fmt.Errorf("get API log config: %w", err)
	}

	apiKVWithDefaults := mergeWithDefaults(string(apiKV), apiHelp)
	status, err := parseLogRecorderAPIStatus(apiKVWithDefaults)
	if err != nil {
		return LogRecorderAPIStatus{}, fmt.Errorf("parse API log config: %w", err)
	}

	return status, nil
}

// GetErrorLogConfig returns the Error log recorder configuration
func (adm *AdminClient) GetErrorLogConfig(ctx context.Context) (LogRecorderErrorStatus, error) {
	errorHelp, err := adm.HelpConfigKV(ctx, ErrorLogRecorderSubSys, "", false)
	if err != nil {
		return LogRecorderErrorStatus{}, fmt.Errorf("get Error log help: %w", err)
	}

	errorKV, err := adm.GetConfigKV(ctx, ErrorLogRecorderSubSys)
	if err != nil {
		return LogRecorderErrorStatus{}, fmt.Errorf("get Error log config: %w", err)
	}

	errorKVWithDefaults := mergeWithDefaults(string(errorKV), errorHelp)
	status, err := parseLogRecorderErrorStatus(errorKVWithDefaults)
	if err != nil {
		return LogRecorderErrorStatus{}, fmt.Errorf("parse Error log config: %w", err)
	}

	return status, nil
}

// GetAuditLogConfig returns the Audit log recorder configuration
func (adm *AdminClient) GetAuditLogConfig(ctx context.Context) (LogRecorderAuditStatus, error) {
	auditHelp, err := adm.HelpConfigKV(ctx, AuditLogRecorderSubSys, "", false)
	if err != nil {
		return LogRecorderAuditStatus{}, fmt.Errorf("get Audit log help: %w", err)
	}

	auditKV, err := adm.GetConfigKV(ctx, AuditLogRecorderSubSys)
	if err != nil {
		return LogRecorderAuditStatus{}, fmt.Errorf("get Audit log config: %w", err)
	}

	auditKVWithDefaults := mergeWithDefaults(string(auditKV), auditHelp)
	status, err := parseLogRecorderAuditStatus(auditKVWithDefaults)
	if err != nil {
		return LogRecorderAuditStatus{}, fmt.Errorf("parse Audit log config: %w", err)
	}

	return status, nil
}

// SetAPILogConfig sets the API log recorder configuration
func (adm *AdminClient) SetAPILogConfig(ctx context.Context, config *LogRecorderAPIConfig) error {
	if config == nil {
		return ErrInvalidArgument("API log configuration cannot be nil")
	}

	kvStr := logRecorderAPIConfigToKV(config)
	if kvStr != "" {
		kvStr = APILogRecorderSubSys + " " + kvStr
		if _, err := adm.SetConfigKV(ctx, kvStr); err != nil {
			return fmt.Errorf("set API log config: %w", err)
		}
	}

	return nil
}

// SetErrorLogConfig sets the Error log recorder configuration
func (adm *AdminClient) SetErrorLogConfig(ctx context.Context, config *LogRecorderErrorConfig) error {
	if config == nil {
		return ErrInvalidArgument("Error log configuration cannot be nil")
	}

	kvStr := logRecorderErrorConfigToKV(config)
	if kvStr != "" {
		kvStr = ErrorLogRecorderSubSys + " " + kvStr
		if _, err := adm.SetConfigKV(ctx, kvStr); err != nil {
			return fmt.Errorf("set Error log config: %w", err)
		}
	}

	return nil
}

// SetAuditLogConfig sets the Audit log recorder configuration
func (adm *AdminClient) SetAuditLogConfig(ctx context.Context, config *LogRecorderAuditConfig) error {
	if config == nil {
		return ErrInvalidArgument("Audit log configuration cannot be nil")
	}

	kvStr := logRecorderAuditConfigToKV(config)
	if kvStr != "" {
		kvStr = AuditLogRecorderSubSys + " " + kvStr
		if _, err := adm.SetConfigKV(ctx, kvStr); err != nil {
			return fmt.Errorf("set Audit log config: %w", err)
		}
	}

	return nil
}

// ResetAPILogConfig resets the API log recorder configuration to defaults
func (adm *AdminClient) ResetAPILogConfig(ctx context.Context) error {
	if _, err := adm.DelConfigKV(ctx, APILogRecorderSubSys); err != nil {
		return fmt.Errorf("reset API log config: %w", err)
	}
	return nil
}

// ResetErrorLogConfig resets the Error log recorder configuration to defaults
func (adm *AdminClient) ResetErrorLogConfig(ctx context.Context) error {
	if _, err := adm.DelConfigKV(ctx, ErrorLogRecorderSubSys); err != nil {
		return fmt.Errorf("reset Error log config: %w", err)
	}
	return nil
}

// ResetAuditLogConfig resets the Audit log recorder configuration to defaults
func (adm *AdminClient) ResetAuditLogConfig(ctx context.Context) error {
	if _, err := adm.DelConfigKV(ctx, AuditLogRecorderSubSys); err != nil {
		return fmt.Errorf("reset Audit log config: %w", err)
	}
	return nil
}

// MarshalYAML implements custom YAML marshaling for LogRecorderAPIStatus
func (s LogRecorderAPIStatus) MarshalYAML() (interface{}, error) {
	// Ensure at least one empty target is present for help text generation
	webhook := s.Webhook
	if len(webhook) == 0 {
		webhook = []WebhookConfig{{}}
	}
	kafka := s.Kafka
	if len(kafka) == 0 {
		kafka = []KafkaConfig{{}}
	}

	return &struct {
		FlushInterval string          `yaml:"flush_interval,omitempty"`
		Enabled       bool            `yaml:"enabled"`
		DriveLimit    string          `yaml:"drive_limit,omitempty"`
		FlushCount    int             `yaml:"flush_count,omitempty"`
		Webhook       []WebhookConfig `yaml:"webhook,omitempty"`
		Kafka         []KafkaConfig   `yaml:"kafka,omitempty"`
	}{
		Enabled:       s.Enabled,
		DriveLimit:    s.DriveLimit,
		FlushCount:    s.FlushCount,
		FlushInterval: s.FlushInterval.String(),
		Webhook:       webhook,
		Kafka:         kafka,
	}, nil
}

// MarshalYAML implements custom YAML marshaling for LogRecorderErrorStatus
func (s LogRecorderErrorStatus) MarshalYAML() (interface{}, error) {
	// Ensure at least one empty target is present for help text generation
	webhook := s.Webhook
	if len(webhook) == 0 {
		webhook = []WebhookConfig{{}}
	}
	kafka := s.Kafka
	if len(kafka) == 0 {
		kafka = []KafkaConfig{{}}
	}

	return &struct {
		FlushInterval string          `yaml:"flush_interval,omitempty"`
		Enabled       bool            `yaml:"enabled"`
		DriveLimit    string          `yaml:"drive_limit,omitempty"`
		FlushCount    int             `yaml:"flush_count,omitempty"`
		Webhook       []WebhookConfig `yaml:"webhook,omitempty"`
		Kafka         []KafkaConfig   `yaml:"kafka,omitempty"`
	}{
		Enabled:       s.Enabled,
		DriveLimit:    s.DriveLimit,
		FlushCount:    s.FlushCount,
		FlushInterval: s.FlushInterval.String(),
		Webhook:       webhook,
		Kafka:         kafka,
	}, nil
}

// MarshalYAML implements custom YAML marshaling for LogRecorderAuditStatus
func (s LogRecorderAuditStatus) MarshalYAML() (interface{}, error) {
	// Ensure at least one empty target is present for help text generation
	webhook := s.Webhook
	if len(webhook) == 0 {
		webhook = []WebhookConfig{{}}
	}
	kafka := s.Kafka
	if len(kafka) == 0 {
		kafka = []KafkaConfig{{}}
	}

	return &struct {
		FlushInterval string          `yaml:"flush_interval,omitempty"`
		Enabled       bool            `yaml:"enabled"`
		FlushCount    int             `yaml:"flush_count,omitempty"`
		Webhook       []WebhookConfig `yaml:"webhook,omitempty"`
		Kafka         []KafkaConfig   `yaml:"kafka,omitempty"`
	}{
		Enabled:       s.Enabled,
		FlushCount:    s.FlushCount,
		FlushInterval: s.FlushInterval.String(),
		Webhook:       webhook,
		Kafka:         kafka,
	}, nil
}

// UnmarshalYAML implements custom YAML unmarshaling for LogRecorderAPIConfig
func (c *LogRecorderAPIConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := &struct {
		FlushInterval *string `yaml:"flush_interval,omitempty"`
		Enable        *bool   `yaml:"enabled,omitempty"`
		DriveLimit    *string `yaml:"drive_limit,omitempty"`
		FlushCount    *int    `yaml:"flush_count,omitempty"`
	}{}

	if err := unmarshal(&aux); err != nil {
		return err
	}

	c.Enable = aux.Enable
	c.DriveLimit = aux.DriveLimit
	c.FlushCount = aux.FlushCount

	if aux.FlushInterval != nil {
		d, err := time.ParseDuration(*aux.FlushInterval)
		if err != nil {
			return err
		}
		c.FlushInterval = &d
	}

	return nil
}

// UnmarshalYAML implements custom YAML unmarshaling for LogRecorderErrorConfig
func (c *LogRecorderErrorConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := &struct {
		FlushInterval *string `yaml:"flush_interval,omitempty"`
		Enable        *bool   `yaml:"enabled,omitempty"`
		DriveLimit    *string `yaml:"drive_limit,omitempty"`
		FlushCount    *int    `yaml:"flush_count,omitempty"`
	}{}

	if err := unmarshal(&aux); err != nil {
		return err
	}

	c.Enable = aux.Enable
	c.DriveLimit = aux.DriveLimit
	c.FlushCount = aux.FlushCount

	if aux.FlushInterval != nil {
		d, err := time.ParseDuration(*aux.FlushInterval)
		if err != nil {
			return err
		}
		c.FlushInterval = &d
	}

	return nil
}

// UnmarshalYAML implements custom YAML unmarshaling for LogRecorderAuditConfig
func (c *LogRecorderAuditConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := &struct {
		FlushInterval *string `yaml:"flush_interval,omitempty"`
		Enable        *bool   `yaml:"enabled,omitempty"`
		FlushCount    *int    `yaml:"flush_count,omitempty"`
	}{}

	if err := unmarshal(&aux); err != nil {
		return err
	}

	c.Enable = aux.Enable
	c.FlushCount = aux.FlushCount

	if aux.FlushInterval != nil {
		d, err := time.ParseDuration(*aux.FlushInterval)
		if err != nil {
			return err
		}
		c.FlushInterval = &d
	}

	return nil
}

// KafkaConfig represents configuration for a Kafka target
type KafkaConfig struct {
	Name    string          `json:"name,omitempty" yaml:"name"` // Optional - auto-generated from index if not provided
	Enable  bool            `json:"enable,omitempty" yaml:"enable"`
	Brokers string          `json:"brokers" yaml:"brokers"`
	Topic   string          `json:"topic" yaml:"topic"`
	Version string          `json:"version,omitempty" yaml:"version"`
	TLS     KafkaTLSConfig  `json:"tls,omitempty" yaml:"tls"`
	SASL    KafkaSASLConfig `json:"sasl,omitempty" yaml:"sasl"`
	Comment string          `json:"comment,omitempty" yaml:"comment,omitempty"`
}

// KafkaTLSConfig represents TLS configuration for Kafka
type KafkaTLSConfig struct {
	Enable        bool   `json:"enable,omitempty" yaml:"enable"`
	SkipVerify    bool   `json:"skip_verify,omitempty" yaml:"skip_verify"`
	ClientAuth    string `json:"client_auth,omitempty" yaml:"client_auth"`
	ClientTLSCert string `json:"client_tls_cert,omitempty" yaml:"client_tls_cert"`
	ClientTLSKey  string `json:"client_tls_key,omitempty" yaml:"client_tls_key"`
}

// KafkaSASLConfig represents SASL authentication configuration for Kafka
type KafkaSASLConfig struct {
	Enable    bool   `json:"enable,omitempty" yaml:"enable"`
	Username  string `json:"username,omitempty" yaml:"username"`
	Password  string `json:"password,omitempty" yaml:"password"`
	Mechanism string `json:"mechanism,omitempty" yaml:"mechanism"`
	Principal string `json:"krb_principal,omitempty" yaml:"krb_principal"`
	Realm     string `json:"krb_realm,omitempty" yaml:"krb_realm"`
	Keytab    string `json:"krb_keytab,omitempty" yaml:"krb_keytab"`
	KrbConfig string `json:"krb_config,omitempty" yaml:"krb_config"`
}

// WebhookConfig represents configuration for a Webhook target
type WebhookConfig struct {
	Name             string `json:"name,omitempty" yaml:"name"` // Optional - auto-generated from index if not provided
	Enable           bool   `json:"enable,omitempty" yaml:"enable"`
	Endpoint         string `json:"endpoint" yaml:"endpoint"`
	AuthToken        string `json:"auth_token,omitempty" yaml:"auth_token"`
	TLSSkipVerify    bool   `json:"tls_skip_verify,omitempty" yaml:"tls_skip_verify"`
	HTTPTimeout      string `json:"http_timeout,omitempty" yaml:"http_timeout"`
	ClientCert       string `json:"client_cert,omitempty" yaml:"client_cert"`
	ClientKey        string `json:"client_key,omitempty" yaml:"client_key"`
	DecompressOnSend bool   `json:"decompress_on_send,omitempty" yaml:"decompress_on_send"`
	Comment          string `json:"comment,omitempty" yaml:"comment,omitempty"`
}

// Helper functions to convert between LogRecorderConfig and KV format

// mergeWithDefaults merges actual config KV string with defaults from Help
func mergeWithDefaults(kvStr string, help Help) string {
	actualKVPairs := parseKVString(kvStr)

	defaultKVPairs := extractDefaultsFromHelp(help)

	for key, defaultValue := range defaultKVPairs {
		if _, exists := actualKVPairs[key]; !exists {
			actualKVPairs[key] = defaultValue
		}
	}

	var kvPairs []string
	for key, value := range actualKVPairs {
		if strings.Contains(value, " ") || strings.Contains(value, ",") {
			kvPairs = append(kvPairs, fmt.Sprintf("%s=\"%s\"", key, value))
		} else {
			kvPairs = append(kvPairs, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(kvPairs, " ")
}

// extractDefaultsFromHelp extracts default values from Help descriptions
func extractDefaultsFromHelp(help Help) map[string]string {
	defaults := make(map[string]string)

	for _, kv := range help.KeysHelp {
		desc := kv.Description

		defaultStart := strings.Index(desc, "(default: '")
		if defaultStart == -1 {
			continue
		}

		defaultStart += len("(default: '")
		defaultEnd := strings.Index(desc[defaultStart:], "')")
		if defaultEnd == -1 {
			continue
		}

		defaultValue := desc[defaultStart : defaultStart+defaultEnd]
		defaults[kv.Key] = defaultValue
	}

	return defaults
}

// kafkaConfigToKV converts a KafkaConfig to KV pairs
func kafkaConfigToKV(kafka KafkaConfig, index int) []string {
	var kvPairs []string

	targetName := kafka.Name
	if targetName == "" {
		targetName = fmt.Sprintf("target%d", index+1)
	}

	if kafka.Enable {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_enable:%s=on", targetName))
	}
	kvPairs = append(kvPairs, fmt.Sprintf("kafka_brokers:%s=%s", targetName, kafka.Brokers))
	kvPairs = append(kvPairs, fmt.Sprintf("kafka_topic:%s=%s", targetName, kafka.Topic))

	if kafka.Version != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_version:%s=%s", targetName, kafka.Version))
	}

	// TLS settings
	if kafka.TLS.Enable {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_tls:%s=on", targetName))
	}
	if kafka.TLS.SkipVerify {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_tls_skip_verify:%s=on", targetName))
	}
	if kafka.TLS.ClientAuth != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_tls_client_auth:%s=%s", targetName, kafka.TLS.ClientAuth))
	}
	if kafka.TLS.ClientTLSCert != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_client_tls_cert:%s=%s", targetName, kafka.TLS.ClientTLSCert))
	}
	if kafka.TLS.ClientTLSKey != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_client_tls_key:%s=%s", targetName, kafka.TLS.ClientTLSKey))
	}

	// SASL settings
	if kafka.SASL.Enable {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_sasl:%s=on", targetName))
	}
	if kafka.SASL.Username != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_sasl_username:%s=%s", targetName, kafka.SASL.Username))
	}
	if kafka.SASL.Password != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_sasl_password:%s=%s", targetName, kafka.SASL.Password))
	}
	if kafka.SASL.Mechanism != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_sasl_mechanism:%s=%s", targetName, kafka.SASL.Mechanism))
	}
	if kafka.SASL.Principal != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_sasl_krb_principal:%s=%s", targetName, kafka.SASL.Principal))
	}
	if kafka.SASL.Realm != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_sasl_krb_realm:%s=%s", targetName, kafka.SASL.Realm))
	}
	if kafka.SASL.Keytab != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_sasl_krb_keytab:%s=%s", targetName, kafka.SASL.Keytab))
	}
	if kafka.SASL.KrbConfig != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("kafka_sasl_krb_config:%s=%s", targetName, kafka.SASL.KrbConfig))
	}

	return kvPairs
}

// webhookConfigToKV converts a WebhookConfig to KV pairs
func webhookConfigToKV(webhook WebhookConfig, index int) []string {
	var kvPairs []string

	targetName := webhook.Name
	if targetName == "" {
		targetName = fmt.Sprintf("target%d", index+1)
	}

	if webhook.Enable {
		kvPairs = append(kvPairs, fmt.Sprintf("webhook_enable:%s=on", targetName))
	}
	kvPairs = append(kvPairs, fmt.Sprintf("webhook_endpoint:%s=%s", targetName, webhook.Endpoint))

	if webhook.AuthToken != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("webhook_auth_token:%s=%s", targetName, webhook.AuthToken))
	}
	if webhook.TLSSkipVerify {
		kvPairs = append(kvPairs, fmt.Sprintf("webhook_tls_skip_verify:%s=on", targetName))
	}
	if webhook.HTTPTimeout != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("webhook_http_timeout:%s=%s", targetName, webhook.HTTPTimeout))
	}
	if webhook.ClientCert != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("webhook_client_cert:%s=%s", targetName, webhook.ClientCert))
	}
	if webhook.ClientKey != "" {
		kvPairs = append(kvPairs, fmt.Sprintf("webhook_client_key:%s=%s", targetName, webhook.ClientKey))
	}
	if webhook.DecompressOnSend {
		kvPairs = append(kvPairs, fmt.Sprintf("webhook_decompress_on_send:%s=on", targetName))
	}

	return kvPairs
}

// logRecorderAPIConfigToKV converts LogRecorderAPIConfig to KV string format
func logRecorderAPIConfigToKV(cfg *LogRecorderAPIConfig) string {
	if cfg == nil {
		return ""
	}

	var kvPairs []string

	if cfg.Enable != nil {
		if *cfg.Enable {
			kvPairs = append(kvPairs, "enable=on")
		} else {
			kvPairs = append(kvPairs, "enable=off")
		}
	}

	if cfg.DriveLimit != nil {
		kvPairs = append(kvPairs, fmt.Sprintf("drive_limit=%s", *cfg.DriveLimit))
	}

	if cfg.FlushCount != nil {
		kvPairs = append(kvPairs, fmt.Sprintf("flush_count=%d", *cfg.FlushCount))
	}

	if cfg.FlushInterval != nil {
		kvPairs = append(kvPairs, fmt.Sprintf("flush_interval=%s", cfg.FlushInterval.String()))
	}

	for i, kafka := range cfg.Kafka {
		kvPairs = append(kvPairs, kafkaConfigToKV(kafka, i)...)
	}

	for i, webhook := range cfg.Webhook {
		kvPairs = append(kvPairs, webhookConfigToKV(webhook, i)...)
	}

	return strings.Join(kvPairs, " ")
}

// logRecorderErrorConfigToKV converts LogRecorderErrorConfig to KV string format
func logRecorderErrorConfigToKV(cfg *LogRecorderErrorConfig) string {
	if cfg == nil {
		return ""
	}

	var kvPairs []string

	if cfg.Enable != nil {
		if *cfg.Enable {
			kvPairs = append(kvPairs, "enable=on")
		} else {
			kvPairs = append(kvPairs, "enable=off")
		}
	}

	if cfg.DriveLimit != nil {
		kvPairs = append(kvPairs, fmt.Sprintf("drive_limit=%s", *cfg.DriveLimit))
	}

	if cfg.FlushCount != nil {
		kvPairs = append(kvPairs, fmt.Sprintf("flush_count=%d", *cfg.FlushCount))
	}

	if cfg.FlushInterval != nil {
		kvPairs = append(kvPairs, fmt.Sprintf("flush_interval=%s", cfg.FlushInterval.String()))
	}

	for i, kafka := range cfg.Kafka {
		kvPairs = append(kvPairs, kafkaConfigToKV(kafka, i)...)
	}

	for i, webhook := range cfg.Webhook {
		kvPairs = append(kvPairs, webhookConfigToKV(webhook, i)...)
	}

	return strings.Join(kvPairs, " ")
}

// logRecorderAuditConfigToKV converts LogRecorderAuditConfig to KV string format
func logRecorderAuditConfigToKV(cfg *LogRecorderAuditConfig) string {
	if cfg == nil {
		return ""
	}

	var kvPairs []string

	if cfg.Enable != nil {
		if *cfg.Enable {
			kvPairs = append(kvPairs, "enable=on")
		} else {
			kvPairs = append(kvPairs, "enable=off")
		}
	}

	if cfg.FlushCount != nil {
		kvPairs = append(kvPairs, fmt.Sprintf("flush_count=%d", *cfg.FlushCount))
	}

	if cfg.FlushInterval != nil {
		kvPairs = append(kvPairs, fmt.Sprintf("flush_interval=%s", cfg.FlushInterval.String()))
	}

	for i, kafka := range cfg.Kafka {
		kvPairs = append(kvPairs, kafkaConfigToKV(kafka, i)...)
	}

	for i, webhook := range cfg.Webhook {
		kvPairs = append(kvPairs, webhookConfigToKV(webhook, i)...)
	}

	return strings.Join(kvPairs, " ")
}

// parseLogRecorderAPIStatus parses KV string format to LogRecorderAPIStatus
func parseLogRecorderAPIStatus(kvStr string) (LogRecorderAPIStatus, error) {
	status := LogRecorderAPIStatus{}

	if kvStr == "" {
		return status, nil
	}

	kvPairs := parseKVString(kvStr)

	for key, value := range kvPairs {
		switch key {
		case "enable":
			status.Enabled = parseBool(value)
		case "drive_limit":
			status.DriveLimit = value
		case "flush_count":
			if count, err := strconv.Atoi(value); err == nil {
				status.FlushCount = count
			}
		case "flush_interval":
			if interval, err := time.ParseDuration(value); err == nil {
				status.FlushInterval = interval
			}
		default:
			// Parse Kafka and Webhook fields (both can be configured)
			parseKafkaField(key, value, &status.Kafka)
			parseWebhookField(key, value, &status.Webhook)
		}
	}

	return status, nil
}

// parseLogRecorderErrorStatus parses KV string format to LogRecorderErrorStatus
func parseLogRecorderErrorStatus(kvStr string) (LogRecorderErrorStatus, error) {
	status := LogRecorderErrorStatus{}

	if kvStr == "" {
		return status, nil
	}

	kvPairs := parseKVString(kvStr)

	for key, value := range kvPairs {
		switch key {
		case "enable":
			status.Enabled = parseBool(value)
		case "drive_limit":
			status.DriveLimit = value
		case "flush_count":
			if count, err := strconv.Atoi(value); err == nil {
				status.FlushCount = count
			}
		case "flush_interval":
			if interval, err := time.ParseDuration(value); err == nil {
				status.FlushInterval = interval
			}
		default:
			// Parse Kafka and Webhook fields (both can be configured)
			parseKafkaField(key, value, &status.Kafka)
			parseWebhookField(key, value, &status.Webhook)
		}
	}

	return status, nil
}

// parseLogRecorderAuditStatus parses KV string format to LogRecorderAuditStatus
func parseLogRecorderAuditStatus(kvStr string) (LogRecorderAuditStatus, error) {
	status := LogRecorderAuditStatus{}

	if kvStr == "" {
		return status, nil
	}

	kvPairs := parseKVString(kvStr)

	for key, value := range kvPairs {
		switch key {
		case "enable":
			status.Enabled = parseBool(value)
		case "flush_count":
			if count, err := strconv.Atoi(value); err == nil {
				status.FlushCount = count
			}
		case "flush_interval":
			if interval, err := time.ParseDuration(value); err == nil {
				status.FlushInterval = interval
			}
		default:
			// Parse Kafka and Webhook fields (both can be configured)
			parseKafkaField(key, value, &status.Kafka)
			parseWebhookField(key, value, &status.Webhook)
		}
	}

	return status, nil
}

// extractTargetName extracts the target name from a key.
// It handles both formats:
//   - "prefix" (no target suffix) -> returns "" (default target)
//   - "prefix:targetName" -> returns "targetName"
//
// Returns (targetName, true) if key matches the prefix, ("", false) otherwise.
func extractTargetName(key, prefix string) (string, bool) {
	if key == prefix {
		return "", true // default target (no name displayed)
	}
	if strings.HasPrefix(key, prefix+":") {
		return strings.TrimPrefix(key, prefix+":"), true
	}
	return "", false
}

// parseKafkaField parses a Kafka-related KV field and updates the targets slice.
// Returns true if the key was a Kafka field, false otherwise.
func parseKafkaField(key, value string, targets *[]KafkaConfig) bool {
	kafkaFields := []struct {
		prefix string
		setter func(*KafkaConfig, string)
	}{
		{"kafka_enable", func(k *KafkaConfig, v string) { k.Enable = parseBool(v) }},
		{"kafka_brokers", func(k *KafkaConfig, v string) { k.Brokers = v }},
		{"kafka_topic", func(k *KafkaConfig, v string) { k.Topic = v }},
		{"kafka_version", func(k *KafkaConfig, v string) { k.Version = v }},
		{"kafka_tls", func(k *KafkaConfig, v string) { k.TLS.Enable = parseBool(v) }},
		{"kafka_tls_skip_verify", func(k *KafkaConfig, v string) { k.TLS.SkipVerify = parseBool(v) }},
		{"kafka_tls_client_auth", func(k *KafkaConfig, v string) { k.TLS.ClientAuth = v }},
		{"kafka_client_tls_cert", func(k *KafkaConfig, v string) { k.TLS.ClientTLSCert = v }},
		{"kafka_client_tls_key", func(k *KafkaConfig, v string) { k.TLS.ClientTLSKey = v }},
		{"kafka_sasl", func(k *KafkaConfig, v string) { k.SASL.Enable = parseBool(v) }},
		{"kafka_sasl_username", func(k *KafkaConfig, v string) { k.SASL.Username = v }},
		{"kafka_sasl_password", func(k *KafkaConfig, v string) { k.SASL.Password = v }},
		{"kafka_sasl_mechanism", func(k *KafkaConfig, v string) { k.SASL.Mechanism = v }},
		{"kafka_sasl_krb_principal", func(k *KafkaConfig, v string) { k.SASL.Principal = v }},
		{"kafka_sasl_krb_realm", func(k *KafkaConfig, v string) { k.SASL.Realm = v }},
		{"kafka_sasl_krb_keytab", func(k *KafkaConfig, v string) { k.SASL.Keytab = v }},
		{"kafka_sasl_krb_config", func(k *KafkaConfig, v string) { k.SASL.KrbConfig = v }},
	}

	for _, field := range kafkaFields {
		if targetName, ok := extractTargetName(key, field.prefix); ok {
			kafka := findOrCreateKafkaTarget(targets, targetName)
			field.setter(kafka, value)
			return true
		}
	}
	return false
}

// parseWebhookField parses a Webhook-related KV field and updates the targets slice.
// Returns true if the key was a Webhook field, false otherwise.
func parseWebhookField(key, value string, targets *[]WebhookConfig) bool {
	webhookFields := []struct {
		prefix string
		setter func(*WebhookConfig, string)
	}{
		{"webhook_enable", func(w *WebhookConfig, v string) { w.Enable = parseBool(v) }},
		{"webhook_endpoint", func(w *WebhookConfig, v string) { w.Endpoint = v }},
		{"webhook_auth_token", func(w *WebhookConfig, v string) { w.AuthToken = v }},
		{"webhook_tls_skip_verify", func(w *WebhookConfig, v string) { w.TLSSkipVerify = parseBool(v) }},
		{"webhook_http_timeout", func(w *WebhookConfig, v string) { w.HTTPTimeout = v }},
		{"webhook_client_cert", func(w *WebhookConfig, v string) { w.ClientCert = v }},
		{"webhook_client_key", func(w *WebhookConfig, v string) { w.ClientKey = v }},
		{"webhook_decompress_on_send", func(w *WebhookConfig, v string) { w.DecompressOnSend = parseBool(v) }},
	}

	for _, field := range webhookFields {
		if targetName, ok := extractTargetName(key, field.prefix); ok {
			webhook := findOrCreateWebhookTarget(targets, targetName)
			field.setter(webhook, value)
			return true
		}
	}
	return false
}

// parseBool parses a boolean string value (case-insensitive).
// Returns true for "on", "true", "yes", "1" and false otherwise.
func parseBool(v string) bool {
	switch strings.ToLower(v) {
	case "on", "true", "yes", "1":
		return true
	default:
		return false
	}
}

// parseKVString parses a KV string like "key1=value1 key2=value2" into a map
func parseKVString(kvStr string) map[string]string {
	result := make(map[string]string)

	if kvStr == "" {
		return result
	}

	parts := strings.Fields(kvStr)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := kv[0]
			value := strings.Trim(kv[1], `"`)
			result[key] = value
		}
	}

	return result
}

// findOrCreateKafkaTarget finds or creates a Kafka target with the given name
func findOrCreateKafkaTarget(targets *[]KafkaConfig, name string) *KafkaConfig {
	for i := range *targets {
		if (*targets)[i].Name == name {
			return &(*targets)[i]
		}
	}

	*targets = append(*targets, KafkaConfig{Name: name})
	return &(*targets)[len(*targets)-1]
}

// findOrCreateWebhookTarget finds or creates a Webhook target with the given name
func findOrCreateWebhookTarget(targets *[]WebhookConfig, name string) *WebhookConfig {
	for i := range *targets {
		if (*targets)[i].Name == name {
			return &(*targets)[i]
		}
	}

	*targets = append(*targets, WebhookConfig{Name: name})
	return &(*targets)[len(*targets)-1]
}

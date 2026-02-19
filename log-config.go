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
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Log recorder config key constants (server-side keys are snake_case)
const (
	logKeyEnable           = "enable"
	logKeyDriveLimit       = "drive_limit"
	logKeyFlushCount       = "flush_count"
	logKeyFlushInterval    = "flush_interval"
	logKeyEndpoint         = "endpoint"
	logKeyAuthToken        = "auth_token"
	logKeyClientCert       = "client_cert"
	logKeyClientKey        = "client_key"
	logKeyProxy            = "proxy"
	logKeyQueueDir         = "queue_dir"
	logKeyQueueSize        = "queue_size"
	logKeyHTTPTimeout      = "http_timeout"
	logKeyTLSSkipVerify    = "tls_skip_verify"
	logKeyBrokers          = "brokers"
	logKeyTopic            = "topic"
	logKeyVersion          = "version"
	logKeyTLS              = "tls"
	logKeyTLSClientAuth    = "tls_client_auth"
	logKeyClientTLSCert    = "client_tls_cert"
	logKeyClientTLSKey     = "client_tls_key"
	logKeySASL             = "sasl"
	logKeySASLUsername     = "sasl_username"
	logKeySASLPassword     = "sasl_password"
	logKeySASLMechanism    = "sasl_mechanism"
	logKeySASLKrbRealm     = "sasl_krb_realm"
	logKeySASLKrbKeytab    = "sasl_krb_keytab"
	logKeySASLKrbConfig    = "sasl_krb_config_path"
	logKeySASLKrbPrincipal = "sasl_krb_principal"
	logKeyName             = "name"

	// Iceberg config keys
	logKeyIcebergEnable         = "iceberg_enable"
	logKeyIcebergWarehouse      = "iceberg_warehouse"
	logKeyIcebergNamespace      = "iceberg_namespace"
	logKeyIcebergTable          = "iceberg_table"
	logKeyIcebergCommitInterval = "iceberg_commit_interval"
	logKeyIcebergWriteInterval  = "iceberg_write_interval"
)

// LogField represents a configuration field with its value and description
type LogField struct {
	Value       string `json:"value" yaml:"value"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// String returns the value of the field
func (f LogField) String() string {
	return f.Value
}

// UnmarshalYAML implements custom YAML unmarshalling for LogField.
// It handles both simple format (enable: off) and full format (enable: {value: off, description: ...})
func (f *LogField) UnmarshalYAML(node *yaml.Node) error {
	// Try simple scalar value first (e.g., "off", "on", "100")
	if node.Kind == yaml.ScalarNode {
		f.Value = node.Value
		return nil
	}

	// Try full struct format
	type logFieldAlias LogField
	return node.Decode((*logFieldAlias)(f))
}

// getDescription finds description for a key from Help
func getDescription(help Help, key string) string {
	for _, kh := range help.KeysHelp {
		if kh.Key == key {
			return kh.Description
		}
	}
	return ""
}

// getLogField creates a LogField from SubsysConfig and Help
// For keys not in Help (like "name"), it uses custom descriptions
func getLogField(sc SubsysConfig, help Help, key string) LogField {
	value, _ := sc.Lookup(key)
	description := getDescription(help, key)

	// Custom descriptions for fields not in HelpConfigKV
	if description == "" {
		switch key {
		case logKeyName:
			description = "name of the target; \"_\" for default; auto-generated as \"target-{i}\" if empty when setting"
		}
	}

	// The server's GetConfigKV API doesn't return "enable=on" explicitly when
	// a subsystem is enabled. If enable is empty, default to "on" since the
	// config exists (meaning the subsystem is configured/enabled).
	if key == logKeyEnable && value == "" {
		value = "on"
	}

	return LogField{
		Value:       value,
		Description: description,
	}
}

// InternalRecorder represents common internal recorder config fields
type InternalRecorder struct {
	Enable        LogField `json:"enable" yaml:"enable"`
	DriveLimit    LogField `json:"driveLimit" yaml:"driveLimit"`
	FlushCount    LogField `json:"flushCount" yaml:"flushCount"`
	FlushInterval LogField `json:"flushInterval" yaml:"flushInterval"`
}

// IcebergConfig represents Iceberg export configuration for API logs
type IcebergConfig struct {
	Enable         LogField `json:"enable" yaml:"enable"`
	Warehouse      LogField `json:"warehouse" yaml:"warehouse"`
	Namespace      LogField `json:"namespace" yaml:"namespace"`
	Table          LogField `json:"table" yaml:"table"`
	CommitInterval LogField `json:"commitInterval" yaml:"commitInterval"`
	WriteInterval  LogField `json:"writeInterval" yaml:"writeInterval"`
}

// InternalAPIRecorder represents internal recorder config for API logs
type InternalAPIRecorder struct {
	InternalRecorder `json:",inline" yaml:",inline"`
	Iceberg          IcebergConfig `json:"iceberg" yaml:"iceberg"`
}

// InternalErrorRecorder represents internal recorder config for Error logs
type InternalErrorRecorder struct {
	InternalRecorder `json:",inline" yaml:",inline"`
}

// InternalAuditRecorder represents internal recorder config for Audit logs
type InternalAuditRecorder struct {
	Enable LogField `json:"enable" yaml:"enable"`
}

// WebhookConfig represents base configuration for a Webhook target
type WebhookConfig struct {
	Name          LogField `json:"name" yaml:"name"`
	Enable        LogField `json:"enable" yaml:"enable"`
	Endpoint      LogField `json:"endpoint" yaml:"endpoint"`
	AuthToken     LogField `json:"authToken" yaml:"authToken"`
	ClientCert    LogField `json:"clientCert" yaml:"clientCert"`
	ClientKey     LogField `json:"clientKey" yaml:"clientKey"`
	Proxy         LogField `json:"proxy" yaml:"proxy"`
	QueueDir      LogField `json:"queueDir" yaml:"queueDir"`
	QueueSize     LogField `json:"queueSize" yaml:"queueSize"`
	HTTPTimeout   LogField `json:"httpTimeout" yaml:"httpTimeout"`
	TLSSkipVerify LogField `json:"tlsSkipVerify" yaml:"tlsSkipVerify"`
}

// APIWebhookConfig represents webhook config for API logs (with batching)
type APIWebhookConfig struct {
	WebhookConfig `json:",inline" yaml:",inline"`
	FlushCount    LogField `json:"flushCount" yaml:"flushCount"`
	FlushInterval LogField `json:"flushInterval" yaml:"flushInterval"`
}

// ErrorWebhookConfig represents webhook config for Error logs (with batching)
type ErrorWebhookConfig struct {
	WebhookConfig `json:",inline" yaml:",inline"`
	FlushCount    LogField `json:"flushCount" yaml:"flushCount"`
	FlushInterval LogField `json:"flushInterval" yaml:"flushInterval"`
}

// AuditWebhookConfig represents webhook config for Audit logs (no batching)
type AuditWebhookConfig struct {
	WebhookConfig `json:",inline" yaml:",inline"`
}

// KafkaTLSConfig represents TLS configuration for Kafka
type KafkaTLSConfig struct {
	Enable        LogField `json:"enable" yaml:"enable"`
	SkipVerify    LogField `json:"skipVerify" yaml:"skipVerify"`
	ClientAuth    LogField `json:"clientAuth" yaml:"clientAuth"`
	ClientTLSCert LogField `json:"clientTLSCert" yaml:"clientTLSCert"`
	ClientTLSKey  LogField `json:"clientTLSKey" yaml:"clientTLSKey"`
}

// KafkaSASLConfig represents SASL configuration for Kafka
type KafkaSASLConfig struct {
	Enable        LogField `json:"enable" yaml:"enable"`
	Username      LogField `json:"username" yaml:"username"`
	Password      LogField `json:"password" yaml:"password"`
	Mechanism     LogField `json:"mechanism" yaml:"mechanism"`
	KrbRealm      LogField `json:"krbRealm" yaml:"krbRealm"`
	KrbKeytab     LogField `json:"krbKeytab" yaml:"krbKeytab"`
	KrbConfigPath LogField `json:"krbConfigPath" yaml:"krbConfigPath"`
	KrbPrincipal  LogField `json:"krbPrincipal" yaml:"krbPrincipal"`
}

// KafkaConfig represents base configuration for a Kafka target
type KafkaConfig struct {
	Name      LogField        `json:"name" yaml:"name"`
	Enable    LogField        `json:"enable" yaml:"enable"`
	Brokers   LogField        `json:"brokers" yaml:"brokers"`
	Topic     LogField        `json:"topic" yaml:"topic"`
	Version   LogField        `json:"version" yaml:"version"`
	TLS       KafkaTLSConfig  `json:"tls" yaml:"tls"`
	SASL      KafkaSASLConfig `json:"sasl" yaml:"sasl"`
	QueueDir  LogField        `json:"queueDir" yaml:"queueDir"`
	QueueSize LogField        `json:"queueSize" yaml:"queueSize"`
}

// APIKafkaConfig represents kafka config for API logs (with batching)
type APIKafkaConfig struct {
	KafkaConfig   `json:",inline" yaml:",inline"`
	FlushCount    LogField `json:"flushCount" yaml:"flushCount"`
	FlushInterval LogField `json:"flushInterval" yaml:"flushInterval"`
}

// ErrorKafkaConfig represents kafka config for Error logs (with batching)
type ErrorKafkaConfig struct {
	KafkaConfig   `json:",inline" yaml:",inline"`
	FlushCount    LogField `json:"flushCount" yaml:"flushCount"`
	FlushInterval LogField `json:"flushInterval" yaml:"flushInterval"`
}

// AuditKafkaConfig represents kafka config for Audit logs (no batching)
type AuditKafkaConfig struct {
	KafkaConfig `json:",inline" yaml:",inline"`
}

// LogRecorderAPIConfig represents configuration for API log type
type LogRecorderAPIConfig struct {
	EnvOverrides []EnvOverride       `json:"envOverrides,omitempty" yaml:"-"`
	Internal     InternalAPIRecorder `json:"internal" yaml:"internal"`
	Webhook      []APIWebhookConfig  `json:"webhook" yaml:"webhook"`
	Kafka        []APIKafkaConfig    `json:"kafka" yaml:"kafka"`
}

// LogRecorderErrorConfig represents configuration for Error log type
type LogRecorderErrorConfig struct {
	EnvOverrides []EnvOverride         `json:"envOverrides,omitempty" yaml:"-"`
	Internal     InternalErrorRecorder `json:"internal" yaml:"internal"`
	Webhook      []ErrorWebhookConfig  `json:"webhook" yaml:"webhook"`
	Kafka        []ErrorKafkaConfig    `json:"kafka" yaml:"kafka"`
}

// LogRecorderAuditConfig represents configuration for Audit log type
type LogRecorderAuditConfig struct {
	EnvOverrides []EnvOverride         `json:"envOverrides,omitempty" yaml:"-"`
	Internal     InternalAuditRecorder `json:"internal" yaml:"internal"`
	Webhook      []AuditWebhookConfig  `json:"webhook" yaml:"webhook"`
	Kafka        []AuditKafkaConfig    `json:"kafka" yaml:"kafka"`
}

// kvBuilder helps build KV strings efficiently
type kvBuilder struct {
	sb strings.Builder
}

func (b *kvBuilder) add(key, value string) {
	if b.sb.Len() > 0 {
		b.sb.WriteString(" ")
	}
	if strings.Contains(value, " ") {
		fmt.Fprintf(&b.sb, "%s=%q", key, value)
	} else {
		fmt.Fprintf(&b.sb, "%s=%s", key, value)
	}
}

func (b *kvBuilder) String() string {
	return b.sb.String()
}

// parseInternalRecorder parses SubsysConfig into InternalRecorder with descriptions from Help
func parseInternalRecorder(sc SubsysConfig, help Help) InternalRecorder {
	return InternalRecorder{
		Enable:        getLogField(sc, help, logKeyEnable),
		DriveLimit:    getLogField(sc, help, logKeyDriveLimit),
		FlushCount:    getLogField(sc, help, logKeyFlushCount),
		FlushInterval: getLogField(sc, help, logKeyFlushInterval),
	}
}

// parseIcebergConfig parses SubsysConfig into IcebergConfig with descriptions from Help
func parseIcebergConfig(sc SubsysConfig, help Help) IcebergConfig {
	return IcebergConfig{
		Enable:         getLogField(sc, help, logKeyIcebergEnable),
		Warehouse:      getLogField(sc, help, logKeyIcebergWarehouse),
		Namespace:      getLogField(sc, help, logKeyIcebergNamespace),
		Table:          getLogField(sc, help, logKeyIcebergTable),
		CommitInterval: getLogField(sc, help, logKeyIcebergCommitInterval),
		WriteInterval:  getLogField(sc, help, logKeyIcebergWriteInterval),
	}
}

// parseInternalAPIRecorder parses SubsysConfig into InternalAPIRecorder with descriptions from Help
func parseInternalAPIRecorder(sc SubsysConfig, help Help) InternalAPIRecorder {
	return InternalAPIRecorder{
		InternalRecorder: parseInternalRecorder(sc, help),
		Iceberg:          parseIcebergConfig(sc, help),
	}
}

// parseInternalErrorRecorder parses SubsysConfig into InternalErrorRecorder with descriptions from Help
func parseInternalErrorRecorder(sc SubsysConfig, help Help) InternalErrorRecorder {
	return InternalErrorRecorder{InternalRecorder: parseInternalRecorder(sc, help)}
}

// parseInternalAuditRecorder parses SubsysConfig into InternalAuditRecorder with descriptions from Help
func parseInternalAuditRecorder(sc SubsysConfig, help Help) InternalAuditRecorder {
	return InternalAuditRecorder{
		Enable: getLogField(sc, help, logKeyEnable),
	}
}

// parseWebhookConfig parses SubsysConfig into WebhookConfig with descriptions from Help
func parseWebhookConfig(sc SubsysConfig, help Help) WebhookConfig {
	cfg := WebhookConfig{
		Name:          getLogField(sc, help, logKeyName),
		Enable:        getLogField(sc, help, logKeyEnable),
		Endpoint:      getLogField(sc, help, logKeyEndpoint),
		AuthToken:     getLogField(sc, help, logKeyAuthToken),
		ClientCert:    getLogField(sc, help, logKeyClientCert),
		ClientKey:     getLogField(sc, help, logKeyClientKey),
		Proxy:         getLogField(sc, help, logKeyProxy),
		QueueDir:      getLogField(sc, help, logKeyQueueDir),
		QueueSize:     getLogField(sc, help, logKeyQueueSize),
		HTTPTimeout:   getLogField(sc, help, logKeyHTTPTimeout),
		TLSSkipVerify: getLogField(sc, help, logKeyTLSSkipVerify),
	}
	// Name comes from sc.Target, not from Lookup; use Default if empty
	cfg.Name.Value = sc.Target
	if cfg.Name.Value == "" {
		cfg.Name.Value = Default
	}
	return cfg
}

// parseAPIWebhookConfig parses SubsysConfig into APIWebhookConfig with descriptions from Help
func parseAPIWebhookConfig(sc SubsysConfig, help Help) APIWebhookConfig {
	return APIWebhookConfig{
		WebhookConfig: parseWebhookConfig(sc, help),
		FlushCount:    getLogField(sc, help, logKeyFlushCount),
		FlushInterval: getLogField(sc, help, logKeyFlushInterval),
	}
}

// parseErrorWebhookConfig parses SubsysConfig into ErrorWebhookConfig with descriptions from Help
func parseErrorWebhookConfig(sc SubsysConfig, help Help) ErrorWebhookConfig {
	return ErrorWebhookConfig{
		WebhookConfig: parseWebhookConfig(sc, help),
		FlushCount:    getLogField(sc, help, logKeyFlushCount),
		FlushInterval: getLogField(sc, help, logKeyFlushInterval),
	}
}

// parseAuditWebhookConfig parses SubsysConfig into AuditWebhookConfig with descriptions from Help
func parseAuditWebhookConfig(sc SubsysConfig, help Help) AuditWebhookConfig {
	return AuditWebhookConfig{WebhookConfig: parseWebhookConfig(sc, help)}
}

// parseKafkaConfig parses SubsysConfig into KafkaConfig with descriptions from Help
func parseKafkaConfig(sc SubsysConfig, help Help) KafkaConfig {
	cfg := KafkaConfig{
		Name:      getLogField(sc, help, logKeyName),
		Enable:    getLogField(sc, help, logKeyEnable),
		Brokers:   getLogField(sc, help, logKeyBrokers),
		Topic:     getLogField(sc, help, logKeyTopic),
		Version:   getLogField(sc, help, logKeyVersion),
		QueueDir:  getLogField(sc, help, logKeyQueueDir),
		QueueSize: getLogField(sc, help, logKeyQueueSize),
		TLS: KafkaTLSConfig{
			Enable:        getLogField(sc, help, logKeyTLS),
			SkipVerify:    getLogField(sc, help, logKeyTLSSkipVerify),
			ClientAuth:    getLogField(sc, help, logKeyTLSClientAuth),
			ClientTLSCert: getLogField(sc, help, logKeyClientTLSCert),
			ClientTLSKey:  getLogField(sc, help, logKeyClientTLSKey),
		},
		SASL: KafkaSASLConfig{
			Enable:        getLogField(sc, help, logKeySASL),
			Username:      getLogField(sc, help, logKeySASLUsername),
			Password:      getLogField(sc, help, logKeySASLPassword),
			Mechanism:     getLogField(sc, help, logKeySASLMechanism),
			KrbRealm:      getLogField(sc, help, logKeySASLKrbRealm),
			KrbKeytab:     getLogField(sc, help, logKeySASLKrbKeytab),
			KrbConfigPath: getLogField(sc, help, logKeySASLKrbConfig),
			KrbPrincipal:  getLogField(sc, help, logKeySASLKrbPrincipal),
		},
	}
	// Name comes from sc.Target, not from Lookup; use Default if empty
	cfg.Name.Value = sc.Target
	if cfg.Name.Value == "" {
		cfg.Name.Value = Default
	}
	return cfg
}

// parseAPIKafkaConfig parses SubsysConfig into APIKafkaConfig with descriptions from Help
func parseAPIKafkaConfig(sc SubsysConfig, help Help) APIKafkaConfig {
	return APIKafkaConfig{
		KafkaConfig:   parseKafkaConfig(sc, help),
		FlushCount:    getLogField(sc, help, logKeyFlushCount),
		FlushInterval: getLogField(sc, help, logKeyFlushInterval),
	}
}

// parseErrorKafkaConfig parses SubsysConfig into ErrorKafkaConfig with descriptions from Help
func parseErrorKafkaConfig(sc SubsysConfig, help Help) ErrorKafkaConfig {
	return ErrorKafkaConfig{
		KafkaConfig:   parseKafkaConfig(sc, help),
		FlushCount:    getLogField(sc, help, logKeyFlushCount),
		FlushInterval: getLogField(sc, help, logKeyFlushInterval),
	}
}

// parseAuditKafkaConfig parses SubsysConfig into AuditKafkaConfig with descriptions from Help
func parseAuditKafkaConfig(sc SubsysConfig, help Help) AuditKafkaConfig {
	return AuditKafkaConfig{KafkaConfig: parseKafkaConfig(sc, help)}
}

// fetchAndParseConfig fetches config for a subsystem and parses it
func (adm *AdminClient) fetchAndParseConfig(ctx context.Context, subSys string) ([]SubsysConfig, error) {
	buf, err := adm.GetConfigKV(ctx, subSys)
	if err != nil {
		return nil, err
	}
	return ParseServerConfigOutput(string(buf))
}

// GetAPILogConfig returns the API log recorder configuration
func (adm *AdminClient) GetAPILogConfig(ctx context.Context) (LogRecorderAPIConfig, error) {
	var cfg LogRecorderAPIConfig

	// Fetch help for internal config
	internalHelp, err := adm.HelpConfigKV(ctx, LogAPIInternalSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogAPIInternalSubSys, err)
	}

	// Fetch internal config
	internalConfigs, err := adm.fetchAndParseConfig(ctx, LogAPIInternalSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogAPIInternalSubSys, err)
	}
	if len(internalConfigs) > 0 {
		cfg.Internal = parseInternalAPIRecorder(internalConfigs[0], internalHelp)
		cfg.EnvOverrides = append(cfg.EnvOverrides, internalConfigs[0].GetEnvOverrides()...)
	}

	// Fetch help for webhook config
	webhookHelp, err := adm.HelpConfigKV(ctx, LogAPIWebhookSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogAPIWebhookSubSys, err)
	}

	// Fetch webhook configs
	webhookConfigs, err := adm.fetchAndParseConfig(ctx, LogAPIWebhookSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogAPIWebhookSubSys, err)
	}
	for _, sc := range webhookConfigs {
		cfg.Webhook = append(cfg.Webhook, parseAPIWebhookConfig(sc, webhookHelp))
		cfg.EnvOverrides = append(cfg.EnvOverrides, sc.GetEnvOverrides()...)
	}

	// Fetch help for kafka config
	kafkaHelp, err := adm.HelpConfigKV(ctx, LogAPIKafkaSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogAPIKafkaSubSys, err)
	}

	// Fetch kafka configs
	kafkaConfigs, err := adm.fetchAndParseConfig(ctx, LogAPIKafkaSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogAPIKafkaSubSys, err)
	}
	for _, sc := range kafkaConfigs {
		cfg.Kafka = append(cfg.Kafka, parseAPIKafkaConfig(sc, kafkaHelp))
		cfg.EnvOverrides = append(cfg.EnvOverrides, sc.GetEnvOverrides()...)
	}

	return cfg, nil
}

// GetErrorLogConfig returns the Error log recorder configuration
func (adm *AdminClient) GetErrorLogConfig(ctx context.Context) (LogRecorderErrorConfig, error) {
	var cfg LogRecorderErrorConfig

	// Fetch help for internal config
	internalHelp, err := adm.HelpConfigKV(ctx, LogErrorInternalSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogErrorInternalSubSys, err)
	}

	// Fetch internal config
	internalConfigs, err := adm.fetchAndParseConfig(ctx, LogErrorInternalSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogErrorInternalSubSys, err)
	}
	if len(internalConfigs) > 0 {
		cfg.Internal = parseInternalErrorRecorder(internalConfigs[0], internalHelp)
		cfg.EnvOverrides = append(cfg.EnvOverrides, internalConfigs[0].GetEnvOverrides()...)
	}

	// Fetch help for webhook config
	webhookHelp, err := adm.HelpConfigKV(ctx, LogErrorWebhookSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogErrorWebhookSubSys, err)
	}

	// Fetch webhook configs
	webhookConfigs, err := adm.fetchAndParseConfig(ctx, LogErrorWebhookSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogErrorWebhookSubSys, err)
	}
	for _, sc := range webhookConfigs {
		cfg.Webhook = append(cfg.Webhook, parseErrorWebhookConfig(sc, webhookHelp))
		cfg.EnvOverrides = append(cfg.EnvOverrides, sc.GetEnvOverrides()...)
	}

	// Fetch help for kafka config
	kafkaHelp, err := adm.HelpConfigKV(ctx, LogErrorKafkaSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogErrorKafkaSubSys, err)
	}

	// Fetch kafka configs
	kafkaConfigs, err := adm.fetchAndParseConfig(ctx, LogErrorKafkaSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogErrorKafkaSubSys, err)
	}
	for _, sc := range kafkaConfigs {
		cfg.Kafka = append(cfg.Kafka, parseErrorKafkaConfig(sc, kafkaHelp))
		cfg.EnvOverrides = append(cfg.EnvOverrides, sc.GetEnvOverrides()...)
	}

	return cfg, nil
}

// GetAuditLogConfig returns the Audit log recorder configuration
func (adm *AdminClient) GetAuditLogConfig(ctx context.Context) (LogRecorderAuditConfig, error) {
	var cfg LogRecorderAuditConfig

	// Fetch help for internal config
	internalHelp, err := adm.HelpConfigKV(ctx, LogAuditInternalSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogAuditInternalSubSys, err)
	}

	// Fetch internal config
	internalConfigs, err := adm.fetchAndParseConfig(ctx, LogAuditInternalSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogAuditInternalSubSys, err)
	}
	if len(internalConfigs) > 0 {
		cfg.Internal = parseInternalAuditRecorder(internalConfigs[0], internalHelp)
		cfg.EnvOverrides = append(cfg.EnvOverrides, internalConfigs[0].GetEnvOverrides()...)
	}

	// Fetch help for webhook config
	webhookHelp, err := adm.HelpConfigKV(ctx, LogAuditWebhookSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogAuditWebhookSubSys, err)
	}

	// Fetch webhook configs
	webhookConfigs, err := adm.fetchAndParseConfig(ctx, LogAuditWebhookSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogAuditWebhookSubSys, err)
	}
	for _, sc := range webhookConfigs {
		cfg.Webhook = append(cfg.Webhook, parseAuditWebhookConfig(sc, webhookHelp))
		cfg.EnvOverrides = append(cfg.EnvOverrides, sc.GetEnvOverrides()...)
	}

	// Fetch help for kafka config
	kafkaHelp, err := adm.HelpConfigKV(ctx, LogAuditKafkaSubSys, "", false)
	if err != nil {
		return cfg, fmt.Errorf("failed to get help for %s: %w", LogAuditKafkaSubSys, err)
	}

	// Fetch kafka configs
	kafkaConfigs, err := adm.fetchAndParseConfig(ctx, LogAuditKafkaSubSys)
	if err != nil {
		return cfg, fmt.Errorf("failed to get %s config: %w", LogAuditKafkaSubSys, err)
	}
	for _, sc := range kafkaConfigs {
		cfg.Kafka = append(cfg.Kafka, parseAuditKafkaConfig(sc, kafkaHelp))
		cfg.EnvOverrides = append(cfg.EnvOverrides, sc.GetEnvOverrides()...)
	}

	return cfg, nil
}

// buildInternalRecorderKV builds the KV string for internal recorder config
func buildInternalRecorderKV(cfg InternalRecorder, subSys string) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	kv.add(logKeyDriveLimit, cfg.DriveLimit.Value)
	kv.add(logKeyFlushCount, cfg.FlushCount.Value)
	kv.add(logKeyFlushInterval, cfg.FlushInterval.Value)
	return subSys + " " + kv.String()
}

// buildInternalAPIKV builds the KV string for internal API recorder config
func buildInternalAPIKV(cfg InternalAPIRecorder) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	kv.add(logKeyDriveLimit, cfg.DriveLimit.Value)
	kv.add(logKeyFlushCount, cfg.FlushCount.Value)
	kv.add(logKeyFlushInterval, cfg.FlushInterval.Value)
	kv.add(logKeyIcebergEnable, cfg.Iceberg.Enable.Value)
	kv.add(logKeyIcebergWarehouse, cfg.Iceberg.Warehouse.Value)
	kv.add(logKeyIcebergNamespace, cfg.Iceberg.Namespace.Value)
	kv.add(logKeyIcebergTable, cfg.Iceberg.Table.Value)
	kv.add(logKeyIcebergCommitInterval, cfg.Iceberg.CommitInterval.Value)
	kv.add(logKeyIcebergWriteInterval, cfg.Iceberg.WriteInterval.Value)
	return LogAPIInternalSubSys + " " + kv.String()
}

// buildInternalErrorKV builds the KV string for internal Error recorder config
func buildInternalErrorKV(cfg InternalErrorRecorder) string {
	return buildInternalRecorderKV(cfg.InternalRecorder, LogErrorInternalSubSys)
}

// buildInternalAuditKV builds the KV string for internal Audit recorder config
func buildInternalAuditKV(cfg InternalAuditRecorder) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	return LogAuditInternalSubSys + " " + kv.String()
}

// buildWebhookKV builds the KV string for webhook config (base fields)
func buildWebhookKV(cfg WebhookConfig, subSys string) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	kv.add(logKeyEndpoint, cfg.Endpoint.Value)
	kv.add(logKeyAuthToken, cfg.AuthToken.Value)
	kv.add(logKeyClientCert, cfg.ClientCert.Value)
	kv.add(logKeyClientKey, cfg.ClientKey.Value)
	kv.add(logKeyProxy, cfg.Proxy.Value)
	kv.add(logKeyQueueDir, cfg.QueueDir.Value)
	kv.add(logKeyQueueSize, cfg.QueueSize.Value)
	kv.add(logKeyHTTPTimeout, cfg.HTTPTimeout.Value)
	kv.add(logKeyTLSSkipVerify, cfg.TLSSkipVerify.Value)

	target := subSys
	if cfg.Name.Value != "" && cfg.Name.Value != Default {
		target = subSys + SubSystemSeparator + cfg.Name.Value
	}
	return target + " " + kv.String()
}

// buildAPIWebhookKV builds the KV string for API webhook config
func buildAPIWebhookKV(cfg APIWebhookConfig) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	kv.add(logKeyEndpoint, cfg.Endpoint.Value)
	kv.add(logKeyAuthToken, cfg.AuthToken.Value)
	kv.add(logKeyClientCert, cfg.ClientCert.Value)
	kv.add(logKeyClientKey, cfg.ClientKey.Value)
	kv.add(logKeyProxy, cfg.Proxy.Value)
	kv.add(logKeyQueueDir, cfg.QueueDir.Value)
	kv.add(logKeyQueueSize, cfg.QueueSize.Value)
	kv.add(logKeyHTTPTimeout, cfg.HTTPTimeout.Value)
	kv.add(logKeyTLSSkipVerify, cfg.TLSSkipVerify.Value)
	kv.add(logKeyFlushCount, cfg.FlushCount.Value)
	kv.add(logKeyFlushInterval, cfg.FlushInterval.Value)

	target := LogAPIWebhookSubSys
	if cfg.Name.Value != "" && cfg.Name.Value != Default {
		target = LogAPIWebhookSubSys + SubSystemSeparator + cfg.Name.Value
	}
	return target + " " + kv.String()
}

// buildErrorWebhookKV builds the KV string for Error webhook config
func buildErrorWebhookKV(cfg ErrorWebhookConfig) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	kv.add(logKeyEndpoint, cfg.Endpoint.Value)
	kv.add(logKeyAuthToken, cfg.AuthToken.Value)
	kv.add(logKeyClientCert, cfg.ClientCert.Value)
	kv.add(logKeyClientKey, cfg.ClientKey.Value)
	kv.add(logKeyProxy, cfg.Proxy.Value)
	kv.add(logKeyQueueDir, cfg.QueueDir.Value)
	kv.add(logKeyQueueSize, cfg.QueueSize.Value)
	kv.add(logKeyHTTPTimeout, cfg.HTTPTimeout.Value)
	kv.add(logKeyTLSSkipVerify, cfg.TLSSkipVerify.Value)
	kv.add(logKeyFlushCount, cfg.FlushCount.Value)
	kv.add(logKeyFlushInterval, cfg.FlushInterval.Value)

	target := LogErrorWebhookSubSys
	if cfg.Name.Value != "" && cfg.Name.Value != Default {
		target = LogErrorWebhookSubSys + SubSystemSeparator + cfg.Name.Value
	}
	return target + " " + kv.String()
}

// buildAuditWebhookKV builds the KV string for Audit webhook config
func buildAuditWebhookKV(cfg AuditWebhookConfig) string {
	return buildWebhookKV(cfg.WebhookConfig, LogAuditWebhookSubSys)
}

// buildKafkaKV builds the KV string for kafka config (base fields)
func buildKafkaKV(cfg KafkaConfig, subSys string) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	kv.add(logKeyBrokers, cfg.Brokers.Value)
	kv.add(logKeyTopic, cfg.Topic.Value)
	kv.add(logKeyVersion, cfg.Version.Value)
	kv.add(logKeyTLS, cfg.TLS.Enable.Value)
	kv.add(logKeyTLSSkipVerify, cfg.TLS.SkipVerify.Value)
	kv.add(logKeyTLSClientAuth, cfg.TLS.ClientAuth.Value)
	kv.add(logKeyClientTLSCert, cfg.TLS.ClientTLSCert.Value)
	kv.add(logKeyClientTLSKey, cfg.TLS.ClientTLSKey.Value)
	kv.add(logKeySASL, cfg.SASL.Enable.Value)
	kv.add(logKeySASLUsername, cfg.SASL.Username.Value)
	kv.add(logKeySASLPassword, cfg.SASL.Password.Value)
	kv.add(logKeySASLMechanism, cfg.SASL.Mechanism.Value)
	kv.add(logKeySASLKrbRealm, cfg.SASL.KrbRealm.Value)
	kv.add(logKeySASLKrbKeytab, cfg.SASL.KrbKeytab.Value)
	kv.add(logKeySASLKrbConfig, cfg.SASL.KrbConfigPath.Value)
	kv.add(logKeySASLKrbPrincipal, cfg.SASL.KrbPrincipal.Value)
	kv.add(logKeyQueueDir, cfg.QueueDir.Value)
	kv.add(logKeyQueueSize, cfg.QueueSize.Value)

	target := subSys
	if cfg.Name.Value != "" && cfg.Name.Value != Default {
		target = subSys + SubSystemSeparator + cfg.Name.Value
	}
	return target + " " + kv.String()
}

// buildAPIKafkaKV builds the KV string for API kafka config
func buildAPIKafkaKV(cfg APIKafkaConfig) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	kv.add(logKeyBrokers, cfg.Brokers.Value)
	kv.add(logKeyTopic, cfg.Topic.Value)
	kv.add(logKeyVersion, cfg.Version.Value)
	kv.add(logKeyTLS, cfg.TLS.Enable.Value)
	kv.add(logKeyTLSSkipVerify, cfg.TLS.SkipVerify.Value)
	kv.add(logKeyTLSClientAuth, cfg.TLS.ClientAuth.Value)
	kv.add(logKeyClientTLSCert, cfg.TLS.ClientTLSCert.Value)
	kv.add(logKeyClientTLSKey, cfg.TLS.ClientTLSKey.Value)
	kv.add(logKeySASL, cfg.SASL.Enable.Value)
	kv.add(logKeySASLUsername, cfg.SASL.Username.Value)
	kv.add(logKeySASLPassword, cfg.SASL.Password.Value)
	kv.add(logKeySASLMechanism, cfg.SASL.Mechanism.Value)
	kv.add(logKeySASLKrbRealm, cfg.SASL.KrbRealm.Value)
	kv.add(logKeySASLKrbKeytab, cfg.SASL.KrbKeytab.Value)
	kv.add(logKeySASLKrbConfig, cfg.SASL.KrbConfigPath.Value)
	kv.add(logKeySASLKrbPrincipal, cfg.SASL.KrbPrincipal.Value)
	kv.add(logKeyQueueDir, cfg.QueueDir.Value)
	kv.add(logKeyQueueSize, cfg.QueueSize.Value)
	kv.add(logKeyFlushCount, cfg.FlushCount.Value)
	kv.add(logKeyFlushInterval, cfg.FlushInterval.Value)

	target := LogAPIKafkaSubSys
	if cfg.Name.Value != "" && cfg.Name.Value != Default {
		target = LogAPIKafkaSubSys + SubSystemSeparator + cfg.Name.Value
	}
	return target + " " + kv.String()
}

// buildErrorKafkaKV builds the KV string for Error kafka config
func buildErrorKafkaKV(cfg ErrorKafkaConfig) string {
	var kv kvBuilder
	kv.add(logKeyEnable, cfg.Enable.Value)
	kv.add(logKeyBrokers, cfg.Brokers.Value)
	kv.add(logKeyTopic, cfg.Topic.Value)
	kv.add(logKeyVersion, cfg.Version.Value)
	kv.add(logKeyTLS, cfg.TLS.Enable.Value)
	kv.add(logKeyTLSSkipVerify, cfg.TLS.SkipVerify.Value)
	kv.add(logKeyTLSClientAuth, cfg.TLS.ClientAuth.Value)
	kv.add(logKeyClientTLSCert, cfg.TLS.ClientTLSCert.Value)
	kv.add(logKeyClientTLSKey, cfg.TLS.ClientTLSKey.Value)
	kv.add(logKeySASL, cfg.SASL.Enable.Value)
	kv.add(logKeySASLUsername, cfg.SASL.Username.Value)
	kv.add(logKeySASLPassword, cfg.SASL.Password.Value)
	kv.add(logKeySASLMechanism, cfg.SASL.Mechanism.Value)
	kv.add(logKeySASLKrbRealm, cfg.SASL.KrbRealm.Value)
	kv.add(logKeySASLKrbKeytab, cfg.SASL.KrbKeytab.Value)
	kv.add(logKeySASLKrbConfig, cfg.SASL.KrbConfigPath.Value)
	kv.add(logKeySASLKrbPrincipal, cfg.SASL.KrbPrincipal.Value)
	kv.add(logKeyQueueDir, cfg.QueueDir.Value)
	kv.add(logKeyQueueSize, cfg.QueueSize.Value)
	kv.add(logKeyFlushCount, cfg.FlushCount.Value)
	kv.add(logKeyFlushInterval, cfg.FlushInterval.Value)

	target := LogErrorKafkaSubSys
	if cfg.Name.Value != "" && cfg.Name.Value != Default {
		target = LogErrorKafkaSubSys + SubSystemSeparator + cfg.Name.Value
	}
	return target + " " + kv.String()
}

// buildAuditKafkaKV builds the KV string for Audit kafka config
func buildAuditKafkaKV(cfg AuditKafkaConfig) string {
	return buildKafkaKV(cfg.KafkaConfig, LogAuditKafkaSubSys)
}

// getCurrentTargets fetches current targets from server for a subsystem
func (adm *AdminClient) getCurrentTargets(ctx context.Context, subSys string) (map[string]bool, error) {
	configs, err := adm.fetchAndParseConfig(ctx, subSys)
	if err != nil {
		return nil, err
	}
	targets := make(map[string]bool)
	for _, sc := range configs {
		name := sc.Target
		if name == "" {
			name = Default
		}
		targets[name] = true
	}
	return targets, nil
}

// deleteRemovedTargets deletes targets that exist on server but not in the new config
func (adm *AdminClient) deleteRemovedTargets(ctx context.Context, subSys string, newNames map[string]bool) error {
	currentTargets, err := adm.getCurrentTargets(ctx, subSys)
	if err != nil {
		return err
	}

	for target := range currentTargets {
		if !newNames[target] && target != Default {
			// Delete this target
			targetKey := subSys + SubSystemSeparator + target
			if _, err := adm.DelConfigKV(ctx, targetKey); err != nil {
				return fmt.Errorf("failed to delete %s: %w", targetKey, err)
			}
		}
	}
	return nil
}

// SetAPILogConfig sets the API log recorder configuration
func (adm *AdminClient) SetAPILogConfig(ctx context.Context, cfg LogRecorderAPIConfig) error {
	// Assign names to unnamed targets using index and check for duplicates
	webhookNames := make(map[string]bool)
	for i := range cfg.Webhook {
		name := cfg.Webhook[i].Name.Value
		if name == "" {
			name = fmt.Sprintf("target-%d", i+1)
			cfg.Webhook[i].Name.Value = name
		}
		if webhookNames[name] {
			return fmt.Errorf("duplicate webhook target name: %s", name)
		}
		webhookNames[name] = true
	}

	kafkaNames := make(map[string]bool)
	for i := range cfg.Kafka {
		name := cfg.Kafka[i].Name.Value
		if name == "" {
			name = fmt.Sprintf("target-%d", i+1)
			cfg.Kafka[i].Name.Value = name
		}
		if kafkaNames[name] {
			return fmt.Errorf("duplicate kafka target name: %s", name)
		}
		kafkaNames[name] = true
	}

	// Delete removed webhook targets
	if err := adm.deleteRemovedTargets(ctx, LogAPIWebhookSubSys, webhookNames); err != nil {
		return err
	}

	// Delete removed kafka targets
	if err := adm.deleteRemovedTargets(ctx, LogAPIKafkaSubSys, kafkaNames); err != nil {
		return err
	}

	// Set internal config
	if _, err := adm.SetConfigKV(ctx, buildInternalAPIKV(cfg.Internal)); err != nil {
		return fmt.Errorf("failed to set %s config: %w", LogAPIInternalSubSys, err)
	}

	// Set webhook configs
	for _, w := range cfg.Webhook {
		if _, err := adm.SetConfigKV(ctx, buildAPIWebhookKV(w)); err != nil {
			return fmt.Errorf("failed to set webhook config for %s: %w", w.Name.Value, err)
		}
	}

	// Set kafka configs
	for _, k := range cfg.Kafka {
		if _, err := adm.SetConfigKV(ctx, buildAPIKafkaKV(k)); err != nil {
			return fmt.Errorf("failed to set kafka config for %s: %w", k.Name.Value, err)
		}
	}

	return nil
}

// SetErrorLogConfig sets the Error log recorder configuration
func (adm *AdminClient) SetErrorLogConfig(ctx context.Context, cfg LogRecorderErrorConfig) error {
	// Assign names to unnamed targets using index and check for duplicates
	webhookNames := make(map[string]bool)
	for i := range cfg.Webhook {
		name := cfg.Webhook[i].Name.Value
		if name == "" {
			name = fmt.Sprintf("target-%d", i+1)
			cfg.Webhook[i].Name.Value = name
		}
		if webhookNames[name] {
			return fmt.Errorf("duplicate webhook target name: %s", name)
		}
		webhookNames[name] = true
	}

	kafkaNames := make(map[string]bool)
	for i := range cfg.Kafka {
		name := cfg.Kafka[i].Name.Value
		if name == "" {
			name = fmt.Sprintf("target-%d", i+1)
			cfg.Kafka[i].Name.Value = name
		}
		if kafkaNames[name] {
			return fmt.Errorf("duplicate kafka target name: %s", name)
		}
		kafkaNames[name] = true
	}

	// Delete removed webhook targets
	if err := adm.deleteRemovedTargets(ctx, LogErrorWebhookSubSys, webhookNames); err != nil {
		return err
	}

	// Delete removed kafka targets
	if err := adm.deleteRemovedTargets(ctx, LogErrorKafkaSubSys, kafkaNames); err != nil {
		return err
	}

	// Set internal config
	if _, err := adm.SetConfigKV(ctx, buildInternalErrorKV(cfg.Internal)); err != nil {
		return fmt.Errorf("failed to set %s config: %w", LogErrorInternalSubSys, err)
	}

	// Set webhook configs
	for _, w := range cfg.Webhook {
		if _, err := adm.SetConfigKV(ctx, buildErrorWebhookKV(w)); err != nil {
			return fmt.Errorf("failed to set webhook config for %s: %w", w.Name.Value, err)
		}
	}

	// Set kafka configs
	for _, k := range cfg.Kafka {
		if _, err := adm.SetConfigKV(ctx, buildErrorKafkaKV(k)); err != nil {
			return fmt.Errorf("failed to set kafka config for %s: %w", k.Name.Value, err)
		}
	}

	return nil
}

// SetAuditLogConfig sets the Audit log recorder configuration
func (adm *AdminClient) SetAuditLogConfig(ctx context.Context, cfg LogRecorderAuditConfig) error {
	// Assign names to unnamed targets using index and check for duplicates
	webhookNames := make(map[string]bool)
	for i := range cfg.Webhook {
		name := cfg.Webhook[i].Name.Value
		if name == "" {
			name = fmt.Sprintf("target-%d", i+1)
			cfg.Webhook[i].Name.Value = name
		}
		if webhookNames[name] {
			return fmt.Errorf("duplicate webhook target name: %s", name)
		}
		webhookNames[name] = true
	}

	kafkaNames := make(map[string]bool)
	for i := range cfg.Kafka {
		name := cfg.Kafka[i].Name.Value
		if name == "" {
			name = fmt.Sprintf("target-%d", i+1)
			cfg.Kafka[i].Name.Value = name
		}
		if kafkaNames[name] {
			return fmt.Errorf("duplicate kafka target name: %s", name)
		}
		kafkaNames[name] = true
	}

	// Delete removed webhook targets
	if err := adm.deleteRemovedTargets(ctx, LogAuditWebhookSubSys, webhookNames); err != nil {
		return err
	}

	// Delete removed kafka targets
	if err := adm.deleteRemovedTargets(ctx, LogAuditKafkaSubSys, kafkaNames); err != nil {
		return err
	}

	// Set internal config
	if _, err := adm.SetConfigKV(ctx, buildInternalAuditKV(cfg.Internal)); err != nil {
		return fmt.Errorf("failed to set %s config: %w", LogAuditInternalSubSys, err)
	}

	// Set webhook configs
	for _, w := range cfg.Webhook {
		if _, err := adm.SetConfigKV(ctx, buildAuditWebhookKV(w)); err != nil {
			return fmt.Errorf("failed to set webhook config for %s: %w", w.Name.Value, err)
		}
	}

	// Set kafka configs
	for _, k := range cfg.Kafka {
		if _, err := adm.SetConfigKV(ctx, buildAuditKafkaKV(k)); err != nil {
			return fmt.Errorf("failed to set kafka config for %s: %w", k.Name.Value, err)
		}
	}

	return nil
}

// ResetAPILogConfig resets API log config to defaults
func (adm *AdminClient) ResetAPILogConfig(ctx context.Context) error {
	// Delete all webhook targets
	if _, err := adm.DelConfigKV(ctx, LogAPIWebhookSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogAPIWebhookSubSys, err)
		}
	}
	// Delete all kafka targets
	if _, err := adm.DelConfigKV(ctx, LogAPIKafkaSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogAPIKafkaSubSys, err)
		}
	}
	// Reset internal config
	if _, err := adm.DelConfigKV(ctx, LogAPIInternalSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogAPIInternalSubSys, err)
		}
	}
	return nil
}

// ResetErrorLogConfig resets Error log config to defaults
func (adm *AdminClient) ResetErrorLogConfig(ctx context.Context) error {
	if _, err := adm.DelConfigKV(ctx, LogErrorWebhookSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogErrorWebhookSubSys, err)
		}
	}
	if _, err := adm.DelConfigKV(ctx, LogErrorKafkaSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogErrorKafkaSubSys, err)
		}
	}
	if _, err := adm.DelConfigKV(ctx, LogErrorInternalSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogErrorInternalSubSys, err)
		}
	}
	return nil
}

// ResetAuditLogConfig resets Audit log config to defaults
func (adm *AdminClient) ResetAuditLogConfig(ctx context.Context) error {
	if _, err := adm.DelConfigKV(ctx, LogAuditWebhookSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogAuditWebhookSubSys, err)
		}
	}
	if _, err := adm.DelConfigKV(ctx, LogAuditKafkaSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogAuditKafkaSubSys, err)
		}
	}
	if _, err := adm.DelConfigKV(ctx, LogAuditInternalSubSys); err != nil {
		if !errors.Is(err, ErrConfigNotFound) {
			return fmt.Errorf("failed to reset %s: %w", LogAuditInternalSubSys, err)
		}
	}
	return nil
}

// ErrConfigNotFound is returned when a config key is not found
var ErrConfigNotFound = errors.New("config key not found")

// writeLogField writes a field with optional comment
func writeLogField(sb *strings.Builder, indent, key string, field LogField) {
	if field.Description != "" {
		sb.WriteString(indent)
		sb.WriteString("# ")
		sb.WriteString(field.Description)
		sb.WriteString("\n")
	}
	sb.WriteString(indent)
	sb.WriteString(key)
	sb.WriteString(": ")
	sb.WriteString(quoteYAMLValue(field.Value))
	sb.WriteString("\n")
}

// writeLogFieldArrayFirst writes the first field of an array item with "- " prefix
// Comment is aligned with the field key (after "- ")
func writeLogFieldArrayFirst(sb *strings.Builder, indent, key string, field LogField) {
	if field.Description != "" {
		sb.WriteString(indent)
		sb.WriteString("  ") // Extra indent to align with key after "- "
		sb.WriteString("# ")
		sb.WriteString(field.Description)
		sb.WriteString("\n")
	}
	sb.WriteString(indent)
	sb.WriteString("- ")
	sb.WriteString(key)
	sb.WriteString(": ")
	sb.WriteString(quoteYAMLValue(field.Value))
	sb.WriteString("\n")
}

// quoteYAMLValue quotes value if needed for valid YAML
func quoteYAMLValue(v string) string {
	if v == "" || strings.ContainsAny(v, " \t\n:#{}[]&*!|>'\"%@`") {
		return fmt.Sprintf("%q", v)
	}
	return v
}

// writeEnvOverrides writes environment variable overrides as YAML comments
func writeEnvOverrides(sb *strings.Builder, envs []EnvOverride) {
	if len(envs) == 0 {
		return
	}
	sb.WriteString("# Environment variables set (take priority over configuration):\n")
	sb.WriteString("# ---\n")
	for _, env := range envs {
		sb.WriteString("# ")
		sb.WriteString(env.Name)
		sb.WriteString("=")
		sb.WriteString(env.Value)
		sb.WriteString("\n")
	}
	sb.WriteString("# ---\n")
}

// YAML returns the configuration as YAML with field descriptions as comments
func (c LogRecorderAPIConfig) YAML() string {
	var sb strings.Builder

	writeEnvOverrides(&sb, c.EnvOverrides)

	sb.WriteString("internal:\n")
	writeLogField(&sb, "  ", "enable", c.Internal.Enable)
	writeLogField(&sb, "  ", "driveLimit", c.Internal.DriveLimit)
	writeLogField(&sb, "  ", "flushCount", c.Internal.FlushCount)
	writeLogField(&sb, "  ", "flushInterval", c.Internal.FlushInterval)
	sb.WriteString("  iceberg:\n")
	writeLogField(&sb, "    ", "enable", c.Internal.Iceberg.Enable)
	writeLogField(&sb, "    ", "warehouse", c.Internal.Iceberg.Warehouse)
	writeLogField(&sb, "    ", "namespace", c.Internal.Iceberg.Namespace)
	writeLogField(&sb, "    ", "table", c.Internal.Iceberg.Table)
	writeLogField(&sb, "    ", "commitInterval", c.Internal.Iceberg.CommitInterval)
	writeLogField(&sb, "    ", "writeInterval", c.Internal.Iceberg.WriteInterval)

	if len(c.Webhook) > 0 {
		sb.WriteString("webhook:\n")
		for _, w := range c.Webhook {
			writeLogFieldArrayFirst(&sb, "  ", "name", w.Name)
			writeLogField(&sb, "    ", "enable", w.Enable)
			writeLogField(&sb, "    ", "endpoint", w.Endpoint)
			writeLogField(&sb, "    ", "authToken", w.AuthToken)
			writeLogField(&sb, "    ", "clientCert", w.ClientCert)
			writeLogField(&sb, "    ", "clientKey", w.ClientKey)
			writeLogField(&sb, "    ", "proxy", w.Proxy)
			writeLogField(&sb, "    ", "queueDir", w.QueueDir)
			writeLogField(&sb, "    ", "queueSize", w.QueueSize)
			writeLogField(&sb, "    ", "httpTimeout", w.HTTPTimeout)
			writeLogField(&sb, "    ", "tlsSkipVerify", w.TLSSkipVerify)
			writeLogField(&sb, "    ", "flushCount", w.FlushCount)
			writeLogField(&sb, "    ", "flushInterval", w.FlushInterval)
		}
	}

	if len(c.Kafka) > 0 {
		sb.WriteString("kafka:\n")
		for _, k := range c.Kafka {
			writeLogFieldArrayFirst(&sb, "  ", "name", k.Name)
			writeLogField(&sb, "    ", "enable", k.Enable)
			writeLogField(&sb, "    ", "brokers", k.Brokers)
			writeLogField(&sb, "    ", "topic", k.Topic)
			writeLogField(&sb, "    ", "version", k.Version)
			sb.WriteString("    tls:\n")
			writeLogField(&sb, "      ", "enable", k.TLS.Enable)
			writeLogField(&sb, "      ", "skipVerify", k.TLS.SkipVerify)
			writeLogField(&sb, "      ", "clientAuth", k.TLS.ClientAuth)
			writeLogField(&sb, "      ", "clientTLSCert", k.TLS.ClientTLSCert)
			writeLogField(&sb, "      ", "clientTLSKey", k.TLS.ClientTLSKey)
			sb.WriteString("    sasl:\n")
			writeLogField(&sb, "      ", "enable", k.SASL.Enable)
			writeLogField(&sb, "      ", "username", k.SASL.Username)
			writeLogField(&sb, "      ", "password", k.SASL.Password)
			writeLogField(&sb, "      ", "mechanism", k.SASL.Mechanism)
			writeLogField(&sb, "      ", "krbRealm", k.SASL.KrbRealm)
			writeLogField(&sb, "      ", "krbKeytab", k.SASL.KrbKeytab)
			writeLogField(&sb, "      ", "krbConfigPath", k.SASL.KrbConfigPath)
			writeLogField(&sb, "      ", "krbPrincipal", k.SASL.KrbPrincipal)
			writeLogField(&sb, "    ", "queueDir", k.QueueDir)
			writeLogField(&sb, "    ", "queueSize", k.QueueSize)
			writeLogField(&sb, "    ", "flushCount", k.FlushCount)
			writeLogField(&sb, "    ", "flushInterval", k.FlushInterval)
		}
	}

	return sb.String()
}

// YAML returns the configuration as YAML with field descriptions as comments
func (c LogRecorderErrorConfig) YAML() string {
	var sb strings.Builder

	writeEnvOverrides(&sb, c.EnvOverrides)

	sb.WriteString("internal:\n")
	writeLogField(&sb, "  ", "enable", c.Internal.Enable)
	writeLogField(&sb, "  ", "driveLimit", c.Internal.DriveLimit)
	writeLogField(&sb, "  ", "flushCount", c.Internal.FlushCount)
	writeLogField(&sb, "  ", "flushInterval", c.Internal.FlushInterval)

	if len(c.Webhook) > 0 {
		sb.WriteString("webhook:\n")
		for _, w := range c.Webhook {
			writeLogFieldArrayFirst(&sb, "  ", "name", w.Name)
			writeLogField(&sb, "    ", "enable", w.Enable)
			writeLogField(&sb, "    ", "endpoint", w.Endpoint)
			writeLogField(&sb, "    ", "authToken", w.AuthToken)
			writeLogField(&sb, "    ", "clientCert", w.ClientCert)
			writeLogField(&sb, "    ", "clientKey", w.ClientKey)
			writeLogField(&sb, "    ", "proxy", w.Proxy)
			writeLogField(&sb, "    ", "queueDir", w.QueueDir)
			writeLogField(&sb, "    ", "queueSize", w.QueueSize)
			writeLogField(&sb, "    ", "httpTimeout", w.HTTPTimeout)
			writeLogField(&sb, "    ", "tlsSkipVerify", w.TLSSkipVerify)
			writeLogField(&sb, "    ", "flushCount", w.FlushCount)
			writeLogField(&sb, "    ", "flushInterval", w.FlushInterval)
		}
	}

	if len(c.Kafka) > 0 {
		sb.WriteString("kafka:\n")
		for _, k := range c.Kafka {
			writeLogFieldArrayFirst(&sb, "  ", "name", k.Name)
			writeLogField(&sb, "    ", "enable", k.Enable)
			writeLogField(&sb, "    ", "brokers", k.Brokers)
			writeLogField(&sb, "    ", "topic", k.Topic)
			writeLogField(&sb, "    ", "version", k.Version)
			sb.WriteString("    tls:\n")
			writeLogField(&sb, "      ", "enable", k.TLS.Enable)
			writeLogField(&sb, "      ", "skipVerify", k.TLS.SkipVerify)
			writeLogField(&sb, "      ", "clientAuth", k.TLS.ClientAuth)
			writeLogField(&sb, "      ", "clientTLSCert", k.TLS.ClientTLSCert)
			writeLogField(&sb, "      ", "clientTLSKey", k.TLS.ClientTLSKey)
			sb.WriteString("    sasl:\n")
			writeLogField(&sb, "      ", "enable", k.SASL.Enable)
			writeLogField(&sb, "      ", "username", k.SASL.Username)
			writeLogField(&sb, "      ", "password", k.SASL.Password)
			writeLogField(&sb, "      ", "mechanism", k.SASL.Mechanism)
			writeLogField(&sb, "      ", "krbRealm", k.SASL.KrbRealm)
			writeLogField(&sb, "      ", "krbKeytab", k.SASL.KrbKeytab)
			writeLogField(&sb, "      ", "krbConfigPath", k.SASL.KrbConfigPath)
			writeLogField(&sb, "      ", "krbPrincipal", k.SASL.KrbPrincipal)
			writeLogField(&sb, "    ", "queueDir", k.QueueDir)
			writeLogField(&sb, "    ", "queueSize", k.QueueSize)
			writeLogField(&sb, "    ", "flushCount", k.FlushCount)
			writeLogField(&sb, "    ", "flushInterval", k.FlushInterval)
		}
	}

	return sb.String()
}

// YAML returns the configuration as YAML with field descriptions as comments
func (c LogRecorderAuditConfig) YAML() string {
	var sb strings.Builder

	writeEnvOverrides(&sb, c.EnvOverrides)

	sb.WriteString("internal:\n")
	writeLogField(&sb, "  ", "enable", c.Internal.Enable)

	if len(c.Webhook) > 0 {
		sb.WriteString("webhook:\n")
		for _, w := range c.Webhook {
			writeLogFieldArrayFirst(&sb, "  ", "name", w.Name)
			writeLogField(&sb, "    ", "enable", w.Enable)
			writeLogField(&sb, "    ", "endpoint", w.Endpoint)
			writeLogField(&sb, "    ", "authToken", w.AuthToken)
			writeLogField(&sb, "    ", "clientCert", w.ClientCert)
			writeLogField(&sb, "    ", "clientKey", w.ClientKey)
			writeLogField(&sb, "    ", "proxy", w.Proxy)
			writeLogField(&sb, "    ", "queueDir", w.QueueDir)
			writeLogField(&sb, "    ", "queueSize", w.QueueSize)
			writeLogField(&sb, "    ", "httpTimeout", w.HTTPTimeout)
			writeLogField(&sb, "    ", "tlsSkipVerify", w.TLSSkipVerify)
		}
	}

	if len(c.Kafka) > 0 {
		sb.WriteString("kafka:\n")
		for _, k := range c.Kafka {
			writeLogFieldArrayFirst(&sb, "  ", "name", k.Name)
			writeLogField(&sb, "    ", "enable", k.Enable)
			writeLogField(&sb, "    ", "brokers", k.Brokers)
			writeLogField(&sb, "    ", "topic", k.Topic)
			writeLogField(&sb, "    ", "version", k.Version)
			sb.WriteString("    tls:\n")
			writeLogField(&sb, "      ", "enable", k.TLS.Enable)
			writeLogField(&sb, "      ", "skipVerify", k.TLS.SkipVerify)
			writeLogField(&sb, "      ", "clientAuth", k.TLS.ClientAuth)
			writeLogField(&sb, "      ", "clientTLSCert", k.TLS.ClientTLSCert)
			writeLogField(&sb, "      ", "clientTLSKey", k.TLS.ClientTLSKey)
			sb.WriteString("    sasl:\n")
			writeLogField(&sb, "      ", "enable", k.SASL.Enable)
			writeLogField(&sb, "      ", "username", k.SASL.Username)
			writeLogField(&sb, "      ", "password", k.SASL.Password)
			writeLogField(&sb, "      ", "mechanism", k.SASL.Mechanism)
			writeLogField(&sb, "      ", "krbRealm", k.SASL.KrbRealm)
			writeLogField(&sb, "      ", "krbKeytab", k.SASL.KrbKeytab)
			writeLogField(&sb, "      ", "krbConfigPath", k.SASL.KrbConfigPath)
			writeLogField(&sb, "      ", "krbPrincipal", k.SASL.KrbPrincipal)
			writeLogField(&sb, "    ", "queueDir", k.QueueDir)
			writeLogField(&sb, "    ", "queueSize", k.QueueSize)
		}
	}

	return sb.String()
}

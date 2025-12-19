module github.com/minio/madmin-go/v4

go 1.25

toolchain go1.25.4

// Install tools using 'go install tool'.
tool (
	github.com/tinylib/msgp
	golang.org/x/tools/cmd/stringer
)

require (
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/dustin/go-humanize v1.0.1
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/minio/madmin-go/v3 v3.0.110
	github.com/minio/minio-go/v7 v7.0.94
	github.com/minio/pkg/v3 v3.4.0
	github.com/prometheus/common v0.67.4
	github.com/prometheus/procfs v0.16.1
	github.com/prometheus/prom2json v1.4.2
	github.com/safchain/ethtool v0.6.1
	github.com/secure-io/sio-go v0.3.1
	github.com/shirou/gopsutil/v4 v4.25.5
	github.com/tinylib/msgp v1.5.0
	golang.org/x/crypto v0.43.0
	golang.org/x/net v0.46.0
	golang.org/x/text v0.30.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/lufia/plan9stats v0.0.0-20250317134145-8bc96cf8fc35 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/minio/crc64nvme v1.0.2 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/prometheus v0.304.1 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	golang.org/x/mod v0.28.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/tools v0.37.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

module github.com/minio/madmin-go/v3

go 1.17

require (
	github.com/cespare/xxhash/v2 v2.2.0
	github.com/dustin/go-humanize v1.0.1
	github.com/minio/minio-go/v7 v7.0.49
	github.com/prometheus/procfs v0.9.0
	github.com/secure-io/sio-go v0.3.1
	github.com/shirou/gopsutil/v3 v3.23.1
	github.com/tinylib/msgp v1.1.8
	golang.org/x/crypto v0.11.0
	golang.org/x/net v0.12.0
)

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/lufia/plan9stats v0.0.0-20230110061619-bbe2e5e100de // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.10.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)

replace github.com/minio/minio-go/v7 => github.com/poornas/minio-go/v7 v7.0.0-20230816201254-210e170bdc4c

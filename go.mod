module github.com/Cray-HPE/hms-power-control

go 1.23

toolchain go1.23.1

//todo hms-base needs to be converted to hms-xname as soon as that package is available.

require (
	github.com/Cray-HPE/hms-base/v2 v2.1.1-0.20250115145051-38c3b67dad0b
	github.com/Cray-HPE/hms-certs v1.3.3
	github.com/Cray-HPE/hms-compcredentials v1.11.2
	github.com/Cray-HPE/hms-hmetcd v1.11.1-0.20250115230651-a066da5f7788
	github.com/Cray-HPE/hms-securestorage v1.13.0
	github.com/Cray-HPE/hms-smd/v2 v2.4.0
	github.com/Cray-HPE/hms-trs-app-api/v3 v3.0.3-0.20250115161119-969732941dcf
	github.com/Cray-HPE/hms-xname v1.4.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/namsral/flag v1.7.4-pre
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/Cray-HPE/hms-base v1.15.1 // indirect
	github.com/Cray-HPE/hms-trs-kafkalib/v2 v2.0.2-0.20250115152934-9169af0750a9 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.8.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.8 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.6 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/vault/api v1.15.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.17 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.17 // indirect
	go.etcd.io/etcd/client/v3 v3.5.17 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.69.4 // indirect
	google.golang.org/protobuf v1.36.3 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

module github.com/Cray-HPE/hms-power-control

go 1.23

toolchain go1.23.1

//todo hms-base needs to be converted to hms-xname as soon as that package is available.

require (
	github.com/Cray-HPE/hms-base v1.15.1
	github.com/Cray-HPE/hms-certs v1.3.3
	github.com/Cray-HPE/hms-compcredentials v1.11.2
	github.com/Cray-HPE/hms-hmetcd v1.10.3
	github.com/Cray-HPE/hms-securestorage v1.13.0
	github.com/Cray-HPE/hms-smd/v2 v2.4.0
	github.com/Cray-HPE/hms-trs-app-api/v3 v3.0.1
	github.com/Cray-HPE/hms-xname v1.3.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/namsral/flag v1.7.4-pre
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.7.2
)

require (
	github.com/Cray-HPE/hms-base/v2 v2.0.1 // indirect
	github.com/Cray-HPE/hms-trs-kafkalib/v2 v2.0.1 // indirect
	github.com/confluentinc/confluent-kafka-go v1.8.2 // indirect
	github.com/coreos/etcd v3.3.13+incompatible // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.1 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/vault/api v1.0.4 // indirect
	github.com/hashicorp/vault/sdk v0.1.13 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.3.0 // indirect
	github.com/pierrec/lz4 v2.4.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	go.etcd.io/etcd v3.3.13+incompatible // indirect
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.56.3 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1 // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

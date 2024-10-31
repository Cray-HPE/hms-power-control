module github.com/Cray-HPE/hms-power-control

go 1.17

//todo hms-base needs to be converted to hms-xname as soon as that package is available.

require (
	github.com/Cray-HPE/hms-base v1.15.1
	github.com/Cray-HPE/hms-certs v1.3.3
	github.com/Cray-HPE/hms-compcredentials v1.11.2
	github.com/Cray-HPE/hms-hmetcd v1.10.3
	github.com/Cray-HPE/hms-securestorage v1.12.2
	github.com/Cray-HPE/hms-smd/v2 v2.4.0
	github.com/Cray-HPE/hms-trs-app-api/v2 v2.1.0
	github.com/Cray-HPE/hms-xname v1.0.0
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/namsral/flag v1.7.4-pre
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.2
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

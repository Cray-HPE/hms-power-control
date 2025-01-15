# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.9.0] - 2025-01-28

### Security

- Update image and module dependencies

## [2.8.0] - 2025-01-08

### Added

- Added support for ppprof builds

## [2.7.0] - 2024-12-16

### Fixed

- Added support for storing large transitions in etcd
- Changed to truncate long messages
- Changed to page large transitions

## [2.6.0] - 2024-11-25

### Fixed

- Updated hms-trs-app-api vendor code (bug fixes and enhancements)
- Passed PCS's log level through to TRS to match PCS's
- Configured TRS to use connection pools for status requests to BMCs
- Renamed PCS_STATUS_HTTP_TIMEOUT to PCS_STATUS_TIMEOUT as it is not an
  http timeout
- Added PCS_MAX_IDLE_CONNS and PCS_MAX_IDLE_CONNS_PER_HOST env variables
  which allow overriding PCS's connection pool settings in TRS
- Added PCS_BASE_TRS_TASK_TIMEOUT env variable which allows the timeout
  for power transitions and capping to be configured
- The above variables are configurable on PCS's helm chart
- At PCS start time, log all env variables that were set
- Added PodName global to facilitate easier debug of log messages
- Log start and end of large batched requests to BMCs
- Fixed many resource leaks associated with making http requests and using TRS
- Update required version of Go to 1.23 to avoid
  https://github.com/golang/go/issues/59017

## [2.5.0] - 2024-10-25

### Changed
### Fixed

- Added ability to configure http status timeout and retries with
  PCS_STATUS_HTTP_TIMEOUT and PCS_STATUS_HTTP_RETRIES env variables
- Updated hms-trs-app-api vendor code to latest version
- Updated GoLang build version from 1.16 to 1.17 so that new vendor code compiles

## [2.4.1] - 2024-10-22

### Fixed

- CASMHMS-6287: Updated API spec to correctly reflect the possible return values for
  power transition operations, and to note that input values for power transition
  operations are not case sensitive.
- Fix broken docker-compose tests because of Github runner changes

### Changed

- CASMHMS-6287: Created `power_operation` and `transition_status` schemas in API spec
  to eliminate redundancies (no semantic change to spec).

## [2.4.0] - 2024-04-24

### Added

- Parse PowerConsumedWatts for any data type and intialize pcap min/max appropriately

## [2.3.0] - 2024-03-26

### Added

- CASMHMS-6156: Added `POST` requests for `/power-status` endpoint, to allow larger
  arguments to be specified.

## [2.2.0] - 2024-03-06

### Fixed

- CASMHMS-6146: Generate correct PowerCapURI for Olympus hardware

## [2.1.0] - 2023-02-27

### Removed

- CASMHMS-6131: Remove unused HSNBoard related code


## [2.0.0] - 2023-09-28

### Fixed

- CASMHMS-6058 - Reduced ETCD storage size of completed power cap tasks and transitions.
- CASMHMS-6058 - Made record expiration and the maximum number of completed records configurable.

## [1.10.0] - 2023-06-08

### Added

- CASMHMS-5995 - Functionality to utilize PowerMaps in HSM when available.

### Fixed

- CASMHMS-5998 - Memory leak in transitions and power-cap domain functions.
- CASMHMS-5996 - power-cap naming mismatch between snapshots and parsed HSM data.

## [1.9.0] - 2023-05-03

### Added

- CASMHMS-5952 - Add MgmtSwitch type components to power status.

## [1.8.0] - 2023-04-07

### Fixed

- CASMHMS-5967 - Remove node 'Ready' restriction for power capping.

## [1.7.0] - 2023-03-28

### Fixed

- CASMHMS-5919 - Fixed issue causing PCS hardware scan to hang.

### Changed

- Increased the log level of status API messages.

## [1.6.0] - 2023-02-28

### Fixed

- Fixed bug in CT tests that use multiple verify response functions

## [1.5.0] - 2023-02-07

### Fixed

- CASMHMS-5917 - Handles /v1/* as well as /*

## [1.4.0] - 2023-02-06

### Fixed

- CASMHMS-5887 - PCS power-status now shows management state 'Unavailable' when it can't communicate with controllers.

## [1.3.0] - 2023-01-31

### Fixed

- CASMHMS-5863 - PCS now reacquires component reservations when transitions get restarted.

## [1.2.0] - 2023-01-31

### Changed

- Updates to non-disruptive CT tests for production systems

## [1.1.0] - 2023-01-26

### Fixed

- CASMHMS-5903: Linting of language in API spec (no content changes); created this changelog file

## [1.0.0] - 2023-01-12

### Added

- Initial release.

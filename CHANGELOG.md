# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

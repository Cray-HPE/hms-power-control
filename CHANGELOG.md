# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.4.3] - 2024-10-22

### Fixed

- CASMHMS-6287: Updated API spec to correctly reflect the possible return values for
  power transition operations, and to note that input values for power transition
  operations are not case sensitive.
- Fix broken docker-compose tests because of Github runner changes

### Changed

- CASMHMS-6287: Created `power_operation` and `transition_status` schemas in API spec
  to eliminate redundancies (no semantic change to spec).

## [1.4.2] - 2023-06-08

### Added

- CASMHMS-5995 - Functionality to utilize PowerMaps in HSM when available.

### Fixed

- CASMHMS-5998 - Memory leak in transitions and power-cap domain functions.

## [1.4.1] - 2023-05-03

### Added

- CASMHMS-5952 - Add MgmtSwitch type components to power status.

### Fixed

- Fixed issue causing PCS hardware scan to hang.

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

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-10-02

### Added

- Initial release.
- Resources: `xyops_alert`, `xyops_api_key`, `xyops_bucket`, `xyops_category`, `xyops_channel`, `xyops_config`, `xyops_event`, `xyops_group`, `xyops_monitor`, `xyops_plugin`, `xyops_role`, `xyops_secret`, `xyops_server`, `xyops_tag`, `xyops_webhook`, `xyops_workflow`.
- Data sources: `xyops_alerts`, `xyops_jobs`, `xyops_servers`.
- `xyops_server` adopts a server xyOps already registered. It does not create or delete servers.
- `xyops_config` manages `job_universal_actions` on the singleton configuration.

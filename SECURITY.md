# Security Policy

## Supported versions

Security fixes are made against the current development branch and the latest published release when practical. Support for older releases is not guaranteed unless a release note says otherwise.

## Hardening

Data-source sessions are opened read-only where the engine supports it, on top of application-layer SELECT validation (`security.enforce_read_only`, #20). See [docs/readonly-access.md](docs/readonly-access.md) for the engine support matrix, per-data-source configuration, and a least-privilege account cookbook. Engines without a session read-only mode rely on SQL validation plus a restricted account — provision read-only credentials for every data source.

## Reporting a vulnerability

Use [GitHub's private vulnerability reporting form](https://github.com/RhinoLink-Tech/data-fabric/security/advisories/new). Do not report exploitable vulnerabilities in a public issue or discussion.

Include the affected version or commit, impact, reproduction steps, and any safe mitigation. Remove passwords, tokens, customer data, and other secrets from reports and logs.

There is no formal response-time SLA yet. Maintainers will coordinate validation, fixes, and disclosure with the reporter.

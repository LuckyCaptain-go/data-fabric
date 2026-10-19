# Data Fabric — A Lightweight Gateway for Heterogeneous Data

<div align="center">
  <img src="assets/data-fabric-icon.png" alt="Data Fabric logo" width="160">
  <p><strong>One lightweight gateway for many data sources. One SQL language for common queries.</strong></p>
  <p><a href="README_zh.md">简体中文</a></p>
  <p>
    <img src="https://img.shields.io/badge/Go-1.26.1+-00ADD8?logo=go" alt="Go 1.26.1+">
    <img src="https://img.shields.io/badge/License-Apache--2.0-blue" alt="Apache 2.0">
    <a href="https://github.com/RhinoLink-Tech/data-fabric/actions/workflows/go.yml"><img src="https://github.com/RhinoLink-Tech/data-fabric/actions/workflows/go.yml/badge.svg" alt="Go CI"></a>
  </p>
</div>

⭐ [Star Data Fabric](https://github.com/RhinoLink-Tech/data-fabric) · 🚀 [Quick Start](#quick-start) · 🤝 [Contribute](CONTRIBUTING.md)

Do your applications still integrate every database, warehouse, lake, object store, and file source separately—each with its own connection setup, SQL differences, and result handling?

Data Fabric puts those sources behind one lightweight, self-hosted gateway. Teams manage connections in one place, while applications use a consistent API and SQL entry point to query registered sources. Supported SQL syntax can be adapted to the target source, reducing the per-source integration work in every application.

## Why Data Fabric

- **One place to manage sources:** Register and maintain data-source connections, inspect their configuration, and diagnose connectivity through a shared gateway.
- **One query experience for applications:** Submit queries and retrieve results through a consistent API. SQL dialect adaptation follows the compatibility scope documented in the [SQL v1 contract](SQL_TRANSLATION_FEATURES.md).
- **Lightweight by default:** Access an individual source through a gateway driver without deploying a separate federation engine. Connect one only when cross-source SQL is needed.
- **Keep data where it belongs:** Data Fabric provides an access layer rather than a data-copy or ETL platform. Federated query traffic and intermediate results still travel between systems.
- **Built for mixed environments:** Use one application-facing entry point across the databases, warehouses, lakehouses, object stores, and files used by an enterprise or cluster.

## SQL Standard and Compatibility

The latest published international SQL standard is [ISO/IEC 9075:2023 (SQL:2023)](https://www.iso.org/standard/76584.html). In practice, database products implement different feature sets and add dialect-specific syntax; a standard alone does not make every query portable. See the SQL conformance notes from [PostgreSQL](https://www.postgresql.org/docs/current/features.html) and [MySQL](https://dev.mysql.com/doc/refman/8.4/en/compatibility.html).

Data Fabric's SQL v1 is a shared, read-only query profile for common application workloads, not a new SQL dialect or a claim of full SQL:2023 conformance. It covers SELECT and UNION queries, projections and basic expressions, WHERE, INNER/LEFT JOIN, GROUP BY and aggregates, HAVING, ORDER BY, and LIMIT/OFFSET. Common scalar and date/time constructs are covered by the documented examples.

For dialect translation, provide sourceDialect to identify the input SQL dialect; Data Fabric translates it to the target source dialect within the [documented compatibility matrix](SQL_TRANSLATION_FEATURES.md). If sourceDialect is omitted, SQL is sent to the target as native SQL without dialect translation. Engine-specific functions, CTEs, window functions, complex types, and time-travel syntax have no shared translation guarantee. Type coercion, NULL ordering, time zones, precision, and other execution semantics remain engine-dependent.

## How It Fits

| Component | Responsibility |
| --- | --- |
| Data Fabric | Application-facing API, source connection management, diagnostics, query routing, and result retrieval |
| Federation engine (optional) | Access multiple systems through catalogs and connectors, then plan and execute cross-source SQL |
| Source systems | Store and manage the actual data |

Data Fabric is a gateway for applications. It does not replace source databases, become a warehouse, or reimplement a distributed query engine.

~~~mermaid
flowchart LR
    App[AI / RAG / BI / Business Applications] --> Gateway[Data Fabric<br/>Unified API and Source Management]
    Gateway --> Drivers[Single-Source Drivers]
    Drivers --> Sources[Databases / Warehouses / Lakehouses / Object Storage / Files]
    Gateway --> Adapter[Query Engine Adapter]
    Adapter --> Engine[External Federation Engine]
    Engine --> Catalogs[Engine Catalogs and Connectors]
    Catalogs --> Sources
~~~

Applications can use a gateway driver for an individual source or send SQL through an external federation engine for cross-catalog queries.

## Capabilities

- **Data-source lifecycle:** Create, inspect, update, activate, deactivate, and delete data sources.
- **Driver registry:** Extend source types through a common driver interface.
- **Connection diagnostics:** Test connections, validate configuration, and inspect connection health and pool statistics.
- **Unified query API:** Submit SQL and retrieve results in batches; streaming-capable drivers can reduce peak memory use for large results.
- **SQL dialect adaptation:** Translate supported query syntax for a target dialect. See the [SQL v1 compatibility matrix](SQL_TRANSLATION_FEATURES.md).
- **Gateway controls:** JWT authentication, rate limiting, configuration validation, and health checks.

## Data Sources

The repository defines representative source types across these categories. Available operations and authentication options depend on the source and its driver.

| Category | Examples |
| --- | --- |
| Relational databases | MySQL, MariaDB, PostgreSQL, Oracle, SQL Server |
| Domestic and compatible databases | OceanBase, TiDB, TDSQL, GaussDB, DaMeng, KingbaseES, GBase, Oscar, OpenGauss |
| Analytical databases | ClickHouse, Doris, StarRocks, Druid, Hologres |
| Cloud warehouses and big-data platforms | Snowflake, Databricks, Redshift, BigQuery, Hive, MaxCompute |
| Lakehouse table formats | Apache Iceberg, Delta Lake, Apache Hudi |
| Object storage and file systems | S3, MinIO, Alibaba OSS, Tencent COS, Azure Blob, HDFS, Apache Ozone |
| File formats | Parquet, ORC, Avro, CSV, JSON, XML, text |

## Quick Start

### Requirements

- Go 1.26.1 or newer.
- A configured and reachable metadata database.
- Network access to the data sources you plan to register.
- A JWT secret and a credential-encryption keyring supplied through environment variables.

### Run Locally

~~~bash
git clone https://github.com/RhinoLink-Tech/data-fabric.git
cd data-fabric
cp configs/config.yaml.example configs/config.yaml
go mod download
~~~

Configure the metadata database in configs/config.yaml. Generate secrets once and store them securely; reuse the same values across restarts.

~~~bash
export JWT_SECRET="<stable secret with at least 32 bytes>"
export DATA_FABRIC_CREDENTIAL_ACTIVE_KEY_ID=dev-v1
export DATA_FABRIC_CREDENTIAL_KEYS='{"dev-v1":"<base64-encoded 32-byte key>"}'
go run ./cmd/server
~~~

Generate the JWT secret with openssl rand -hex 32 and the credential key with openssl rand -base64 32. Store both securely and reuse them across restarts. The service health-check endpoint is /health. See the [configuration guide](docs/configuration.md) for deployment settings, secret handling, and key rotation.

## API Overview

| Method | Path | Purpose | Authentication |
| --- | --- | --- | --- |
| GET | /health | Service health check | None |
| GET | /api/v1/database/types | List source types recognized by the service | None |
| GET | /api/v1/database/compatibility | Inspect database compatibility information | None |
| GET / POST | /api/v1/datasources | List or create data sources | Bearer when authentication is enabled |
| GET / PUT / DELETE | /api/v1/datasources/:id | Inspect, update, or delete a data source | Bearer when authentication is enabled |
| POST | /api/v1/datasources/:id/activate | Activate a data source | Bearer when authentication is enabled |
| POST | /api/v1/datasources/:id/deactivate | Deactivate a data source | Bearer when authentication is enabled |
| POST | /api/v1/database/test-connection | Test a data-source connection | Bearer when authentication is enabled |
| POST | /api/v1/database/validate-config | Validate data-source configuration | Bearer when authentication is enabled |
| POST | /api/v1/fetch | Run a query and retrieve its first result batch | Bearer (always) |
| GET | /api/v1/fetch/:query_id/:slug/:token | Retrieve a subsequent result batch | Bearer (always) |

See the [API usage guide](docs/api_usage.md) for request fields, response structures, and authentication behavior.

## Documentation

- [API Usage Guide](docs/api_usage.md)
- [Architecture Guide](docs/architecture.md)
- [Configuration Guide](docs/configuration.md)
- [Development Guide](docs/development.md)
- [Contributing Guide](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [Support](SUPPORT.md)

## Cross-Source Query Engine Integrations

When cross-source SQL is needed, Data Fabric can connect to compatible federation engines through a shared adapter. Trino, Presto, and OpenLooKeng can be registered as data sources; their coordinators use configured catalogs and connectors to plan and execute cross-catalog queries. Applications continue to submit SQL through the Data Fabric query API.

See the [API usage guide](docs/api_usage.md) for connection setup and the [SQL v1 contract](SQL_TRANSLATION_FEATURES.md) for the supported SQL compatibility scope.

## Contribute

Issues, pull requests, and documentation improvements are welcome. See the [contribution guide](CONTRIBUTING.md) to get involved.

## License

Apache License 2.0. See [LICENSE](LICENSE).

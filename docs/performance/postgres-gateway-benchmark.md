# PostgreSQL Gateway Query Benchmark

**Run date:** 2026-10-19  
**Database:** PostgreSQL on the developer's local WSL environment (localhost:5432)  
**Workload:** 20 warmups, 1,000 serial samples per path, no concurrent load

## Summary

For the 500-row synthetic result, the in-process Data Fabric query service measured **0.563 ms P50** and **0.969 ms P95**. The direct PostgreSQL path, using the same SQL and inferring the total from the fetched page, measured **0.327 ms P50** and **0.577 ms P95**.

The differences between those reported percentile values are **0.236 ms at P50** and **0.392 ms at P95**. These are differences between separately measured latency distributions, not a guarantee that every individual request adds exactly that amount. The gateway path's measured P95 was below 1 ms in this particular local run.

## Results

All latency values are milliseconds. “Direct, inferred total” avoids a separate count query when all rows fit in the requested page, matching the gateway behavior for these workloads. “Direct, count query” always performs a separate COUNT query.

| Workload | Path | P50 | P95 | Serial rate |
| --- | --- | ---: | ---: | ---: |
| Small constant query | Direct, inferred total | 0.149 | 0.251 | 6,198.9 ops/s |
| Small constant query | Direct, count query | 0.301 | 0.505 | 3,072.9 ops/s |
| Small constant query | Data Fabric query service | 0.397 | 0.674 | 2,319.1 ops/s |
| 500 synthetic rows | Direct, inferred total | 0.327 | 0.577 | 2,784.5 ops/s |
| 500 synthetic rows | Direct, count query | 0.535 | 0.800 | 1,756.7 ops/s |
| 500 synthetic rows | Data Fabric query service | 0.563 | 0.969 | 1,617.2 ops/s |

For the 500-row workload, the P50/P95 value gaps between Data Fabric and direct access with inferred totals are **0.236 ms / 0.392 ms**. Against direct access that always executes a count query, the gaps are **0.028 ms / 0.169 ms**. That second comparison does extra database work on the direct path, so it is useful for showing the count-query cost, but it is not the primary like-for-like comparison.

The reported serial rate is the number of sequential operations per second in this microbenchmark. It is not a concurrent capacity or production QPS claim.

## Method and Scope

- Uses a real PostgreSQL server and Data Fabric's registered PostgreSQL driver; it does not simulate the PostgreSQL protocol.
- Creates or reuses a dedicated data_fabric_perf database and a uniquely named table with 500 rows. Each payload is 128 characters.
- Runs two workloads: SELECT 1 with a 10-row page, and a 500-row ordered table scan with a page size of 510 so the complete result fits on the first page.
- Compares direct PostgreSQL access with the in-process Data Fabric QueryService. Both paths fetch and JSON-encode results; the gateway path also runs its query validation, source lookup, row scanning and type normalization.
- Uses 20 warmups and 1,000 sequential samples per path. There is no concurrent load.
- Does not include the HTTP router, authentication middleware, or the client-to-gateway network hop. The metadata repository is stubbed for this isolated service benchmark.
- The benchmark removes and verifies removal of its temporary table. It leaves the dedicated benchmark database in place for later runs.
- Results are specific to this hardware, local database, data shape and run. They should not be generalized to other engines or production topologies.

## Reproduce

The PostgreSQL account must be able to connect to the admin database and create data_fabric_perf if it does not already exist. Set the password in the shell environment; do not commit credentials.

~~~bash
export PGHOST=localhost
export PGPORT=5432
export PGUSER=your-postgres-user
export PGPASSWORD=your-postgres-password
export PGSSLMODE=disable
export DATA_FABRIC_PERF_ADMIN_DB=postgres
export DATA_FABRIC_PERF_SAMPLES=1000
export DATA_FABRIC_PERF_WARMUPS=20
export DATA_FABRIC_PERF_ROWS=500

go run ./scripts/performance/postgres_gateway_benchmark.go
~~~

The command prints P50, P95 and serial operation rates for each path, then confirms that the temporary table was removed. See the [benchmark program](../../scripts/performance/postgres_gateway_benchmark.go) for implementation details.

## Regression Coverage

The query-service test in the [query routing tests](../../internal/service/query_engine_routing_test.go) verifies that a non-full first page returns an exact total without issuing a count query, while a full page still issues the count query to preserve exact totals. The standalone PostgreSQL benchmark complements this deterministic test with a real local source.

# PostgreSQL Gateway Query Benchmark

**Run date:** 2026-10-19
**Database:** PostgreSQL 17.11 on the local WSL environment
**Runtime:** Go 1.26.1, linux/amd64, 24 visible CPUs
**Dataset:** synthetic tables with 500 and 5,000 rows; local PostgreSQL is the simulated source

## Summary

The 500-row run covered 14 query shapes, with 20 warmups and 1,000 serial samples per path. The in-process Data Fabric query service had P95 latencies from **0.466 ms to 1.929 ms**. The difference between the separately measured direct PostgreSQL and gateway P95 values ranged from **0.281 ms to 0.436 ms**; the P50 value gaps ranged from **0.192 ms to 0.311 ms**.

A second 5,000-row run used 5 warmups and 100 serial samples per path. It provides a scale check rather than a stable capacity claim: the service P95 ranged from **0.454 ms to 17.351 ms**, with the largest value on an unindexed text sort. A wide projection returned 5,000 rows with a 512-character payload and measured 8.735 ms at P95.

A percentile value gap is the difference between two independently measured percentile values. It is not a per-request latency guarantee. The 5,000-row run has fewer samples and should be rerun on the target deployment before using it for capacity planning.

## Environment and Method

- Uses a real local PostgreSQL 17.11 instance as the simulated source, populated with generated synthetic data, and Data Fabric's registered PostgreSQL driver. The database server and driver are real; the benchmark data is simulated.
- Creates an isolated benchmark database if needed, then creates a synthetic fact table with a primary key on **id**, a 128-character text payload, and a dimension table containing about half as many rows. The payload column has no index.
- Compares direct PostgreSQL access with the in-process Data Fabric **QueryService**. Both paths run the same workload SQL, fetch the first result batch, scan rows, and JSON-encode a response. The gateway path also performs query validation, source lookup, routing, result normalization, and its database-pool path.
- The full-page case intentionally fills the first page and checks exact-count behavior; the direct path and service both perform the extra count query.
- Workloads run serially. The direct and gateway paths alternate sample order to reduce order bias. There is no concurrent client load.
- The metadata repository is stubbed for this isolated service test. The test excludes the HTTP router, authentication middleware, and the client-to-gateway network hop.
- The temporary fact and dimension tables are dropped and their removal is verified. The dedicated **data_fabric_perf** database remains available for later runs.
- Results depend on hardware, PostgreSQL settings, data shape, connection locality, and concurrent workload. They are measurements of this local setup, not production guarantees.

## Workload Matrix

| Workload | Query behavior |
| --- | --- |
| small_constant | Small constant SELECT |
| point_lookup | Primary-key equality lookup |
| selective_filter | Selective range filter, ordered by primary key |
| broad_filter | Broad range filter, ordered by primary key |
| ordered_scan | Full scan ordered by primary key |
| sort_unindexed_text | Full scan sorted on an unindexed text column |
| offset_page | Ordered LIMIT and OFFSET page |
| wide_projection | Fourfold payload projection to exercise wider results |
| aggregate_statistics | COUNT, SUM, and AVG |
| grouped_aggregate | Grouped COUNT and SUM |
| distinct_values | DISTINCT over a low-cardinality expression |
| inner_join | Fact-to-dimension inner join |
| left_join | Fact-to-dimension left join with unmatched fact rows |
| full_page_count | Exactly full first page, requiring an exact count |

## Results: 500 Rows

All values are milliseconds. Each cell is P50 / P95. “Gap” is the Data Fabric value minus the direct PostgreSQL value at the same percentile.

| Workload | Direct P50 / P95 | Data Fabric P50 / P95 | Gap P50 / P95 |
| --- | ---: | ---: | ---: |
| small_constant | 0.115 / 0.184 | 0.307 / 0.466 | 0.192 / 0.281 |
| point_lookup | 0.155 / 0.231 | 0.370 / 0.550 | 0.215 / 0.319 |
| selective_filter | 0.178 / 0.277 | 0.417 / 0.604 | 0.239 / 0.327 |
| broad_filter | 0.312 / 0.483 | 0.547 / 0.828 | 0.235 / 0.345 |
| ordered_scan | 0.316 / 0.482 | 0.524 / 0.839 | 0.209 / 0.357 |
| sort_unindexed_text | 1.311 / 1.548 | 1.585 / 1.929 | 0.275 / 0.381 |
| offset_page | 0.267 / 0.401 | 0.511 / 0.756 | 0.243 / 0.355 |
| wide_projection | 0.564 / 0.951 | 0.876 / 1.387 | 0.311 / 0.436 |
| aggregate_statistics | 0.155 / 0.216 | 0.379 / 0.546 | 0.224 / 0.330 |
| grouped_aggregate | 0.208 / 0.300 | 0.458 / 0.634 | 0.250 / 0.334 |
| distinct_values | 0.168 / 0.239 | 0.387 / 0.546 | 0.220 / 0.307 |
| inner_join | 0.423 / 0.578 | 0.710 / 0.977 | 0.287 / 0.398 |
| left_join | 0.419 / 0.650 | 0.699 / 0.980 | 0.280 / 0.330 |
| full_page_count | 0.349 / 0.482 | 0.568 / 0.770 | 0.219 / 0.288 |

## Results: 5,000 Rows

This run used 100 samples per path, so treat it as a directional scale check. All values are milliseconds, formatted as P50 / P95.

| Workload | Direct P50 / P95 | Data Fabric P50 / P95 | Gap P50 / P95 |
| --- | ---: | ---: | ---: |
| small_constant | 0.115 / 0.161 | 0.329 / 0.454 | 0.214 / 0.294 |
| point_lookup | 0.208 / 0.286 | 0.453 / 0.685 | 0.245 / 0.399 |
| selective_filter | 0.414 / 0.685 | 0.650 / 1.061 | 0.236 / 0.376 |
| broad_filter | 2.052 / 2.798 | 2.604 / 3.590 | 0.552 / 0.792 |
| ordered_scan | 1.830 / 2.700 | 2.353 / 3.442 | 0.524 / 0.742 |
| sort_unindexed_text | 15.052 / 17.049 | 15.716 / 17.351 | 0.665 / 0.303 |
| offset_page | 1.027 / 1.572 | 1.423 / 2.051 | 0.396 / 0.479 |
| wide_projection | 5.444 / 6.767 | 6.479 / 8.735 | 1.035 / 1.968 |
| aggregate_statistics | 0.301 / 0.409 | 0.533 / 0.771 | 0.232 / 0.362 |
| grouped_aggregate | 0.801 / 1.028 | 1.104 / 1.342 | 0.303 / 0.314 |
| distinct_values | 0.536 / 0.710 | 0.806 / 1.011 | 0.270 / 0.301 |
| inner_join | 2.132 / 2.629 | 2.564 / 3.258 | 0.431 / 0.629 |
| left_join | 2.382 / 3.094 | 3.084 / 3.941 | 0.702 / 0.847 |
| full_page_count | 0.783 / 1.139 | 0.985 / 1.493 | 0.202 / 0.354 |

## Which Standard Benchmarks Fit?

- **TPC-H** is the best first reference for an analytical query engine: its decision-support workload contains 22 business-oriented queries with different joins, aggregations, filters, and data volumes. A supported subset or adapted harness should be described as “TPC-H-derived,” not an official TPC-H result.
- **TPC-DS** is a broader and more complex decision-support benchmark. It is a stronger later-stage comparison, but requires more schema, query-dialect, data-generation, and execution work.
- **PostgreSQL pgbench** is useful for PostgreSQL connection concurrency and transaction workloads. Its built-in workload is TPC-B-like, not an analytical SQL suite. It speaks PostgreSQL's wire protocol, so it benchmarks a PostgreSQL endpoint directly; Data Fabric's HTTP API needs an HTTP load tool or a dedicated adapter for end-to-end concurrency testing.
- The 14-case matrix in this report is a custom microbenchmark, not TPC-compliant. It measures the Data Fabric query service against a real PostgreSQL source.

Official references: [TPC current specifications](https://tpc.org/TPC_Documents_Current_Versions/current_specifications5.asp), [TPC-H v3.0.1](https://www.tpc.org/TPC_Documents_Current_Versions/pdf/TPC-H_v3.0.1.pdf), [TPC-DS v4.0.0](https://www.tpc.org/TPC_Documents_Current_Versions/pdf/TPC-DS_v4.0.0.pdf), and [PostgreSQL pgbench documentation](https://www.postgresql.org/docs/current/pgbench.html).

## Reproduce

The PostgreSQL account needs permission to connect to the admin database and create **data_fabric_perf** if it does not already exist. Set the password through the shell environment; never put real credentials in the repository.

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

For the scale run, set **DATA_FABRIC_PERF_SAMPLES=100**, **DATA_FABRIC_PERF_WARMUPS=5**, and **DATA_FABRIC_PERF_ROWS=5000**. The program prints the PostgreSQL and Go versions, P50/P95 latency for each path, percentile value gaps, and verifies that temporary tables were removed.

## Regression Coverage

The [query routing tests](../../internal/service/query_engine_routing_test.go) verify that a non-full first page returns an exact total without a count query, while a full page still runs a count query to preserve exact totals. The benchmark complements that deterministic coverage with a real local PostgreSQL source.

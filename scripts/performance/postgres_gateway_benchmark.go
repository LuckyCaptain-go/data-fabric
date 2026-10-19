package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/RhinoLink-Tech/data-fabric/internal/database"
	"github.com/RhinoLink-Tech/data-fabric/internal/model"
	"github.com/RhinoLink-Tech/data-fabric/internal/repository"
	"github.com/RhinoLink-Tech/data-fabric/internal/service"
)

const (
	sourceID = "11111111-1111-4111-8111-111111111111"
	testDB   = "data_fabric_perf"
)

type sourceRepo struct {
	repository.DataSourceRepository
	source *model.DataSource
}

func (r sourceRepo) GetByID(_ context.Context, id string) (*model.DataSource, error) {
	if id != r.source.ID {
		return nil, repository.ErrDataSourceNotFound
	}
	return r.source, nil
}

type stat struct {
	vals  []time.Duration
	total time.Duration
}

func (s *stat) measure(fn func() error) error {
	start := time.Now()
	err := fn()
	d := time.Since(start)
	s.vals = append(s.vals, d)
	s.total += d
	return err
}
func (s stat) report() (time.Duration, time.Duration, float64) {
	v := append([]time.Duration(nil), s.vals...)
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	if len(v) == 0 {
		return 0, 0, 0
	}
	p95 := int(math.Ceil(float64(len(v))*.95)) - 1
	if p95 < 0 {
		p95 = 0
	}
	return v[(len(v)-1)/2], v[p95], float64(len(v)) / s.total.Seconds()
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func envInt(k string, d int) int {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	n, e := strconv.Atoi(v)
	if e != nil {
		return d
	}
	return n
}

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "benchmark:", e)
		os.Exit(1)
	}
}
func run() error {
	host := env("PGHOST", "localhost")
	port := envInt("PGPORT", 5432)
	user := env("PGUSER", "postgres")
	pass := os.Getenv("PGPASSWORD")
	ssl := env("PGSSLMODE", "disable")
	adminDB := env("DATA_FABRIC_PERF_ADMIN_DB", "postgres")
	n := envInt("DATA_FABRIC_PERF_SAMPLES", 100)
	warm := envInt("DATA_FABRIC_PERF_WARMUPS", 5)
	rowsN := envInt("DATA_FABRIC_PERF_ROWS", 500)
	if n < 20 || warm < 1 || rowsN < 1 || rowsN > 9000 {
		return errors.New("samples >=20, warmups >=1, rows 1..9000 required")
	}
	if ssl != "disable" && ssl != "require" {
		return fmt.Errorf("current driver supports PGSSLMODE=disable or require, got %q", ssl)
	}
	cfg := model.DataSourceConfig{Host: host, Port: port, Database: adminDB, Username: user, Password: pass, SSL: ssl == "require", MaxPoolSize: 4, Timeout: 30}
	drv, e := database.GetDriverRegistry().GetDriver(model.DatabaseTypePostgreSQL)
	if e != nil {
		return e
	}
	admin, e := drv.Open(drv.BuildDSN(&cfg))
	if e != nil {
		return e
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e = admin.PingContext(ctx); e != nil {
		return fmt.Errorf("connect as %q: %w", user, e)
	}
	var exists bool
	if e = admin.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", testDB).Scan(&exists); e != nil {
		return e
	}
	created := false
	if !exists {
		if _, e = admin.Exec("CREATE DATABASE " + testDB); e != nil {
			return fmt.Errorf("create %s (account needs CREATEDB): %w", testDB, e)
		}
		created = true
	}
	cfg.Database = testDB
	src := &model.DataSource{ID: sourceID, Name: "temporary-perf-source", Type: model.DatabaseTypePostgreSQL, Status: model.DataSourceStatusActive, Config: cfg}
	direct, e := drv.Open(drv.BuildDSN(&cfg))
	if e != nil {
		return e
	}
	defer direct.Close()
	direct.SetMaxOpenConns(4)
	direct.SetMaxIdleConns(4)
	if e = direct.PingContext(ctx); e != nil {
		return e
	}
	tableName := fmt.Sprintf("df_perf_rows_%d", time.Now().UnixNano())
	if _, e = direct.Exec("CREATE TABLE " + tableName + " (id BIGINT PRIMARY KEY, payload TEXT NOT NULL)"); e != nil {
		return fmt.Errorf("create isolated benchmark table: %w", e)
	}
	defer func() {
		if _, cleanupErr := direct.Exec("DROP TABLE IF EXISTS " + tableName); cleanupErr != nil {
			fmt.Fprintf(os.Stderr, "temporary table cleanup failed: %v\n", cleanupErr)
		}
	}()
	if _, e = direct.Exec(fmt.Sprintf("INSERT INTO %s SELECT g, repeat('x',128) FROM generate_series(1,%d) AS g", tableName, rowsN)); e != nil {
		return fmt.Errorf("populate isolated benchmark table: %w", e)
	}
	pool := database.NewConnectionPool(nil, nil, true)
	defer pool.CloseAll()
	qs := service.NewQueryService(sourceRepo{source: src}, pool, false)
	workloads := []struct {
		name, sql string
		batch     int
	}{
		{"small", "SELECT 1 AS id, 'data-fabric' AS payload", 10},
		{"synthetic_rows", "SELECT id, payload FROM " + tableName + " ORDER BY id", rowsN + 10},
	}
	fmt.Printf("database=%s created_now=%t rows=%d samples=%d warmups=%d; temporary synthetic table is removed after the run\n\n", testDB, created, rowsN, n, warm)
	for _, w := range workloads {
		directRun := func(withCount bool) func() error {
			return func() error {
				total := int64(0)
				if withCount {
					if e := direct.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM ("+w.sql+") AS df_count").Scan(&total); e != nil {
						return e
					}
				}
				rs, e := direct.QueryContext(context.Background(), fmt.Sprintf("%s LIMIT %d", w.sql, w.batch))
				if e != nil {
					return e
				}
				defer rs.Close()
				cols, e := rs.Columns()
				if e != nil {
					return e
				}
				data := make([][]any, 0, w.batch)
				for rs.Next() {
					v := make([]any, len(cols))
					ptr := make([]any, len(cols))
					for i := range v {
						ptr[i] = &v[i]
					}
					if e = rs.Scan(ptr...); e != nil {
						return e
					}
					for i, x := range v {
						if b, ok := x.([]byte); ok {
							v[i] = string(b)
						}
					}
					data = append(data, v)
				}
				if e = rs.Err(); e != nil {
					return e
				}
				if !withCount && len(data) < w.batch {
					// The direct path can infer the exact total when it fetched the
					// complete result set in this page, matching the gateway behavior.
					total = int64(len(data))
				}
				_, e = json.Marshal(map[string]any{"columns": cols, "rows": data, "totalCount": total})
				return e
			}
		}
		gateway := func() error {
			r, e := qs.FetchQuery(context.Background(), &model.FetchQueryRequest{DataSourceID: sourceID, SQL: w.sql, BatchSize: w.batch, Type: 1, Timeout: 60})
			if e != nil {
				return e
			}
			if len(r.Entries) < w.batch && r.TotalCount != len(r.Entries) {
				return fmt.Errorf("gateway totalCount=%d, want %d", r.TotalCount, len(r.Entries))
			}
			_, e = json.Marshal(r)
			return e
		}
		names := []string{"direct_smart_count", "direct_with_count", "gateway_service"}
		ops := []func() error{directRun(false), directRun(true), gateway}
		for i := 0; i < warm; i++ {
			for j, f := range ops {
				if e := f(); e != nil {
					return fmt.Errorf("%s warmup %d: %w", names[j], i, e)
				}
			}
		}
		got := map[string]*stat{}
		for _, name := range names {
			got[name] = &stat{}
		}
		for i := 0; i < n; i++ {
			for k := 0; k < 3; k++ {
				j := (k + i) % 3
				if e := got[names[j]].measure(ops[j]); e != nil {
					return fmt.Errorf("%s sample: %w", names[j], e)
				}
			}
		}
		fmt.Printf("[%s]\n", w.name)
		for _, name := range names {
			p50, p95, tps := got[name].report()
			fmt.Printf("  %-20s p50=%s p95=%s throughput=%.1f ops/s\n", name, p50.Round(time.Microsecond), p95.Round(time.Microsecond), tps)
		}
		g50, g95, _ := got["gateway_service"].report()
		d50, d95, _ := got["direct_with_count"].report()
		b50, b95, _ := got["direct_smart_count"].report()
		fmt.Printf("  gateway vs direct always-count: p50=%+.1f%% p95=%+.1f%%\n", delta(g50, d50), delta(g95, d95))
		fmt.Printf("  gateway vs direct smart-count:  p50=%+.1f%% p95=%+.1f%%\n\n", delta(g50, b50), delta(g95, b95))
	}
	if _, e = direct.Exec("DROP TABLE IF EXISTS " + tableName); e != nil {
		return fmt.Errorf("drop temporary benchmark table: %w", e)
	}
	var tableRemains bool
	if e = direct.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_tables WHERE schemaname = current_schema() AND tablename = $1)", tableName).Scan(&tableRemains); e != nil {
		return fmt.Errorf("verify temporary table cleanup: %w", e)
	}
	if tableRemains {
		return errors.New("temporary benchmark table remains after cleanup")
	}
	fmt.Println("Temporary synthetic table removed and verified.")
	fmt.Println("Scope: in-process query service; excludes HTTP, auth middleware, and client-to-gateway network hop.")
	fmt.Println("direct_smart_count infers totalCount when all results fit in one page; direct_with_count always runs a separate COUNT query.")
	return nil
}
func delta(a, b time.Duration) float64 {
	if b <= 0 {
		return 0
	}
	return 100 * (float64(a)/float64(b) - 1)
}

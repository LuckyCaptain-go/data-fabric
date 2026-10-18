package database

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"sync"
	"testing"

	"github.com/RhinoLink-Tech/data-fabric/internal/model"
)

// failingDriver is a database/sql driver whose connections always fail, used to
// simulate a pooled connection that has gone stale.
type failingDriver struct{}

func (failingDriver) Open(string) (sqldriver.Conn, error) {
	return nil, errors.New("connection refused")
}

type failingConnector struct{}

func (failingConnector) Connect(context.Context) (sqldriver.Conn, error) {
	return nil, errors.New("connection refused")
}

func (failingConnector) Driver() sqldriver.Driver { return failingDriver{} }

// deadDB returns a *sql.DB that is already in the pool but can no longer reach
// its database, so PingContext reports it as dead.
func deadDB() *sql.DB {
	return sql.OpenDB(failingConnector{})
}

// A pooled connection that fails its liveness ping must be evicted and replaced
// without panicking. The stale entry is removed and the replacement attempt
// fails, which has to surface as an error.
func TestGetConnectionEvictsDeadPooledConnection(t *testing.T) {
	const unregisteredType = model.DatabaseType("__unregistered_for_test__")

	pool := NewConnectionPool(nil, nil, false)
	pool.pools["ds-1"] = deadDB()

	dataSource := &model.DataSource{
		ID:     "ds-1",
		Type:   unregisteredType,
		Config: model.DataSourceConfig{Host: "127.0.0.1", Port: 3306, Database: "demo"},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetConnection panicked while evicting a dead pooled connection: %v", r)
		}
	}()

	if _, err := pool.GetConnection(context.Background(), dataSource); err == nil {
		t.Fatal("GetConnection returned a dead pooled connection instead of an error")
	}

	if _, stillPooled := pool.pools["ds-1"]; stillPooled {
		t.Error("the dead connection is still in the pool after GetConnection")
	}
}

// getConnectUrl and ApplyBatchPagination must not depend on some earlier caller
// having initialized the package-level driver registry.
func TestRegistryDependentHelpersWorkOnColdStart(t *testing.T) {
	registryOnce = sync.Once{}
	globalDriverRegistry = nil

	pool := NewConnectionPool(nil, nil, false)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("helper panicked before the driver registry was initialized: %v", r)
		}
	}()

	dsn, err := pool.getConnectUrl(model.DatabaseTypeMySQL, &model.DataSourceConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		Database: "demo",
	})
	if err != nil {
		t.Fatalf("getConnectUrl failed on a cold start: %v", err)
	}
	if dsn == "" {
		t.Error("getConnectUrl returned an empty DSN on a cold start")
	}

	paginated, err := pool.ApplyBatchPagination(model.DatabaseTypeMySQL, "SELECT 1", 10, 0)
	if err != nil {
		t.Fatalf("ApplyBatchPagination failed on a cold start: %v", err)
	}
	if paginated == "SELECT 1" {
		t.Errorf("ApplyBatchPagination returned the query unchanged: %q", paginated)
	}
}

// Read-only enforcement is the global flag ANDed with a per-data-source
// override that can only opt out, never re-enable (#20).
func TestReadOnlyEnforced(t *testing.T) {
	tests := []struct {
		name   string
		global bool
		props  map[string]interface{}
		want   bool
	}{
		{"global off", false, nil, false},
		{"global on without override", true, nil, true},
		{"bool false opts out", true, map[string]interface{}{"enforce_read_only": false}, false},
		{"bool true stays enforced", true, map[string]interface{}{"enforce_read_only": true}, true},
		{"string false opts out", true, map[string]interface{}{"enforce_read_only": "false"}, false},
		{"string off opts out", true, map[string]interface{}{"enforce_read_only": "off"}, false},
		{"string 0 opts out", true, map[string]interface{}{"enforce_read_only": "0"}, false},
		{"unrecognized string stays enforced", true, map[string]interface{}{"enforce_read_only": "banana"}, true},
		{"unrelated properties ignored", true, map[string]interface{}{"pool_hint": false}, true},
		// A per-data-source override can only weaken enforcement; the global
		// flag is the release-mode lock (#20).
		{"global off cannot be re-enabled per data source", false, map[string]interface{}{"enforce_read_only": true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewConnectionPool(nil, nil, tt.global)
			config := &model.DataSourceConfig{AdditionalProps: tt.props}
			if got := pool.readOnlyEnforced(config); got != tt.want {
				t.Errorf("readOnlyEnforced() = %v, want %v", got, tt.want)
			}
		})
	}
}

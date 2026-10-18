package database

import (
	"testing"

	"github.com/RhinoLink-Tech/data-fabric/internal/model"
)

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
			dataSource := &model.DataSource{
				Type:   model.DatabaseTypeMySQL,
				Config: model.DataSourceConfig{AdditionalProps: tt.props},
			}
			if got := pool.readOnlyEnforced(dataSource); got != tt.want {
				t.Errorf("readOnlyEnforced() = %v, want %v", got, tt.want)
			}
		})
	}
}

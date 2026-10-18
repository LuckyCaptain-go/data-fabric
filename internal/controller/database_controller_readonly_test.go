package controller

import (
	"strings"
	"testing"

	"github.com/RhinoLink-Tech/data-fabric/internal/model"
)

func TestGetConfigWarningsReadOnlyAdvisories(t *testing.T) {
	dc := &DatabaseController{}

	tests := []struct {
		name       string
		dbType     model.DatabaseType
		props      map[string]interface{}
		wantSubstr []string
		wantNone   []string
	}{
		{
			name:       "unsupported type gets least-privilege advisory",
			dbType:     model.DatabaseTypeOracle,
			props:      nil,
			wantSubstr: []string{"no database-side read-only session mode"},
			wantNone:   []string{"disabled for this data source"},
		},
		{
			name:       "enforced type gets no read-only advisory",
			dbType:     model.DatabaseTypeMySQL,
			props:      nil,
			wantSubstr: nil,
			wantNone:   []string{"read-only session"},
		},
		{
			name:       "opted-out data source gets disabled advisory",
			dbType:     model.DatabaseTypeMySQL,
			props:      map[string]interface{}{"enforce_read_only": false},
			wantSubstr: []string{"read-only session enforcement is disabled"},
			wantNone:   []string{"no database-side read-only session mode"},
		},
		{
			name:       "string opt-out recognized",
			dbType:     model.DatabaseTypePostgreSQL,
			props:      map[string]interface{}{"enforce_read_only": "off"},
			wantSubstr: []string{"read-only session enforcement is disabled"},
		},
		{
			name:       "explicit opt-in adds no advisory",
			dbType:     model.DatabaseTypePostgreSQL,
			props:      map[string]interface{}{"enforce_read_only": true},
			wantSubstr: nil,
			wantNone:   []string{"read-only session"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &model.DataSourceConfig{
				SSL:             true,
				AdditionalProps: tt.props,
			}
			warnings := dc.getConfigWarnings(config, tt.dbType)
			joined := strings.Join(warnings, "\n")
			for _, want := range tt.wantSubstr {
				if !strings.Contains(joined, want) {
					t.Errorf("getConfigWarnings() = %v, want a warning containing %q", warnings, want)
				}
			}
			for _, banned := range tt.wantNone {
				if strings.Contains(joined, banned) {
					t.Errorf("getConfigWarnings() = %v, unexpectedly contains %q", warnings, banned)
				}
			}
		})
	}
}

package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RhinoLink-Tech/data-fabric/internal/database"
	"github.com/gin-gonic/gin"
)

func TestValidateDataSourceConfigUsesRegisteredTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewDatabaseController(nil, database.NewHealthChecker(nil))

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "registered driver outside the former allowlist",
			body:       `{"type":"sqlserver","config":{"host":"sql.example.test","database":"analytics","username":"gateway"}}`,
			wantStatus: http.StatusOK,
			wantBody:   `"valid":true`,
		},
		{
			name:       "unregistered driver is rejected",
			body:       `{"type":"not_registered","config":{"host":"sql.example.test","database":"analytics","username":"gateway"}}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "INVALID_TYPE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/database/validate-config", strings.NewReader(tt.body))
			controller.ValidateDataSourceConfig(ctx)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; response: %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("response = %s, want it to contain %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

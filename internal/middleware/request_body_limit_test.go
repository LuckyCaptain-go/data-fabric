package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestBodyLimitRejectsOversizedBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const limit = 8

	for _, unknownLength := range []bool{false, true} {
		name := "content length"
		if unknownLength {
			name = "unknown content length"
		}
		t.Run(name, func(t *testing.T) {
			router := gin.New()
			router.POST("/", RequestBodyLimit(limit), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", limit+1)))
			if unknownLength {
				request.ContentLength = -1
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "REQUEST_TOO_LARGE") {
				t.Fatalf("response missing stable error code: %s", recorder.Body.String())
			}
		})
	}
}

func TestRequestBodyLimitReplaysAcceptedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/", RequestBodyLimit(16), func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, "text/plain", body)
	})

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Body.String() != "payload" {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), "payload")
	}
}

func TestRequestBodyLimitLeavesGETRequestsAlone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/", RequestBodyLimit(1), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", strings.NewReader("large continuation body"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/RhinoLink-Tech/data-fabric/pkg/response"
	"github.com/gin-gonic/gin"
)

const DefaultMaxRequestBodyBytes int64 = 1 << 20

// RequestBodyLimit buffers only the small JSON request bodies required by
// these API handlers so oversized payloads can receive a stable 413 response.
// It is attached to individual JSON routes; GET batch continuations and
// response streaming are left untouched.
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxRequestBodyBytes
	}

	return func(c *gin.Context) {
		request := c.Request
		if request.Method == http.MethodGet || request.Method == http.MethodHead ||
			request.Body == nil || request.Body == http.NoBody || request.ContentLength == 0 {
			c.Next()
			return
		}

		if request.ContentLength > maxBytes {
			_ = request.Body.Close()
			writeRequestTooLarge(c)
			return
		}

		body, err := io.ReadAll(io.LimitReader(request.Body, maxBytes+1))
		if err != nil {
			_ = request.Body.Close()
			c.AbortWithStatusJSON(http.StatusBadRequest, response.ErrorResponse(
				"INVALID_REQUEST",
				"Unable to read request body",
				"",
				requestCorrelationID(c),
			))
			return
		}
		_ = request.Body.Close()

		if int64(len(body)) > maxBytes {
			writeRequestTooLarge(c)
			return
		}

		request.Body = io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))
		c.Next()
	}
}

func writeRequestTooLarge(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, response.ErrorResponse(
		"REQUEST_TOO_LARGE",
		"Request body exceeds the configured maximum size",
		"",
		requestCorrelationID(c),
	))
}

func requestCorrelationID(c *gin.Context) string {
	correlationID, _ := c.Get(CorrelationIDKey)
	id, _ := correlationID.(string)
	return id
}

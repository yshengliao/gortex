package metrics

import (
	"time"

	"github.com/yshengliao/gortex/http/context"
	"github.com/yshengliao/gortex/middleware"
)

// MetricsMiddleware creates a Gortex middleware for collecting HTTP metrics
func MetricsMiddleware(collector MetricsCollector) middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			start := time.Now()

			// Record request size if available
			if c.Request().ContentLength > 0 {
				collector.RecordHTTPRequestSize(c.Request().Method, c.Request().URL.Path, c.Request().ContentLength)
			}

			// Process request
			err := next(c)

			// Calculate duration
			duration := time.Since(start)

			// Get status code
			statusCode := c.Response().Status()

			// Record metrics
			collector.RecordHTTPRequest(c.Request().Method, c.Request().URL.Path, statusCode, duration)

			// Record response size
			if size := c.Response().Size(); size > 0 {
				collector.RecordHTTPResponseSize(c.Request().Method, c.Request().URL.Path, size)
			}

			return err
		}
	}
}
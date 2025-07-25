package abtest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDualExecutor(t *testing.T) {
	t.Run("identical responses", func(t *testing.T) {
		// Create two identical Echo apps
		echoApp := echo.New()
		gortexApp := echo.New()

		// Register identical handlers
		handler := func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{
				"message": "Hello, World!",
			})
		}
		echoApp.GET("/hello", handler)
		gortexApp.GET("/hello", handler)

		// Create dual executor
		executor := NewDualExecutor(echoApp, gortexApp)

		// Execute request
		req := httptest.NewRequest(http.MethodGet, "/hello", nil)
		result, err := executor.Execute(req)
		require.NoError(t, err)

		// Verify results
		assert.True(t, result.IsIdentical)
		assert.Empty(t, result.Differences)
		assert.Equal(t, http.StatusOK, result.EchoResponse.StatusCode)
		assert.Equal(t, http.StatusOK, result.GortexResponse.StatusCode)

		// Verify response bodies
		var echoBody, gortexBody map[string]string
		err = json.Unmarshal(result.EchoResponse.Body, &echoBody)
		require.NoError(t, err)
		err = json.Unmarshal(result.GortexResponse.Body, &gortexBody)
		require.NoError(t, err)
		assert.Equal(t, echoBody, gortexBody)
	})

	t.Run("different status codes", func(t *testing.T) {
		// Create two Echo apps with different handlers
		echoApp := echo.New()
		gortexApp := echo.New()

		echoApp.GET("/status", func(c echo.Context) error {
			return c.NoContent(http.StatusOK)
		})
		gortexApp.GET("/status", func(c echo.Context) error {
			return c.NoContent(http.StatusCreated)
		})

		// Create dual executor
		executor := NewDualExecutor(echoApp, gortexApp)

		// Execute request
		result, err := executor.ExecuteRequest(http.MethodGet, "/status", nil)
		require.NoError(t, err)

		// Verify results
		assert.False(t, result.IsIdentical)
		assert.Len(t, result.Differences, 1)
		assert.Equal(t, "status", result.Differences[0].Type)
		assert.Equal(t, http.StatusOK, result.Differences[0].EchoValue)
		assert.Equal(t, http.StatusCreated, result.Differences[0].GortexValue)
	})

	t.Run("different headers", func(t *testing.T) {
		// Create two Echo apps with different headers
		echoApp := echo.New()
		gortexApp := echo.New()

		echoApp.GET("/headers", func(c echo.Context) error {
			c.Response().Header().Set("X-Custom-Header", "echo-value")
			return c.String(http.StatusOK, "OK")
		})
		gortexApp.GET("/headers", func(c echo.Context) error {
			c.Response().Header().Set("X-Custom-Header", "gortex-value")
			c.Response().Header().Set("X-Extra-Header", "extra")
			return c.String(http.StatusOK, "OK")
		})

		// Create dual executor
		executor := NewDualExecutor(echoApp, gortexApp)

		// Execute request
		result, err := executor.ExecuteRequest(http.MethodGet, "/headers", nil)
		require.NoError(t, err)

		// Verify results
		assert.False(t, result.IsIdentical)
		
		// Find header differences
		var customHeaderDiff, extraHeaderDiff *Difference
		for i := range result.Differences {
			if result.Differences[i].Field == "x-custom-header" {
				customHeaderDiff = &result.Differences[i]
			} else if result.Differences[i].Field == "x-extra-header" {
				extraHeaderDiff = &result.Differences[i]
			}
		}

		assert.NotNil(t, customHeaderDiff)
		assert.NotNil(t, extraHeaderDiff)
		assert.Equal(t, "header", customHeaderDiff.Type)
		assert.Equal(t, "header", extraHeaderDiff.Type)
	})

	t.Run("different JSON body", func(t *testing.T) {
		// Create two Echo apps with different JSON responses
		echoApp := echo.New()
		gortexApp := echo.New()

		echoApp.GET("/json", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"version": "1.0",
				"data": map[string]string{
					"key": "value1",
				},
			})
		})
		gortexApp.GET("/json", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"version": "1.1",
				"data": map[string]string{
					"key": "value2",
				},
			})
		})

		// Create dual executor
		executor := NewDualExecutor(echoApp, gortexApp)

		// Execute request
		result, err := executor.ExecuteRequest(http.MethodGet, "/json", nil)
		require.NoError(t, err)

		// Verify results
		assert.False(t, result.IsIdentical)
		assert.Greater(t, len(result.Differences), 0)
		
		// Find JSON difference
		var jsonDiff *Difference
		for i := range result.Differences {
			if result.Differences[i].Type == "body" && result.Differences[i].Field == "json" {
				jsonDiff = &result.Differences[i]
				break
			}
		}
		assert.NotNil(t, jsonDiff)
	})

	t.Run("POST request with body", func(t *testing.T) {
		// Create two Echo apps
		echoApp := echo.New()
		gortexApp := echo.New()

		// Handler that echoes the request body
		handler := func(c echo.Context) error {
			var body map[string]interface{}
			if err := c.Bind(&body); err != nil {
				return err
			}
			return c.JSON(http.StatusOK, body)
		}
		echoApp.POST("/echo", handler)
		gortexApp.POST("/echo", handler)

		// Create dual executor
		executor := NewDualExecutor(echoApp, gortexApp)

		// Execute request with body
		reqBody := map[string]string{
			"name": "test",
			"type": "example",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		result, err := executor.Execute(req)
		require.NoError(t, err)

		// Verify results
		assert.True(t, result.IsIdentical)
		assert.Equal(t, bodyBytes, result.RequestBody)
		
		// Verify both responses echo the same body
		var echoResp, gortexResp map[string]interface{}
		err = json.Unmarshal(result.EchoResponse.Body, &echoResp)
		require.NoError(t, err)
		err = json.Unmarshal(result.GortexResponse.Body, &gortexResp)
		require.NoError(t, err)
		assert.Equal(t, echoResp, gortexResp)
	})
}

func TestComparisonRecorder(t *testing.T) {
	t.Run("records all results", func(t *testing.T) {
		recorder := NewComparisonRecorder()

		// Add some results
		results := []ComparisonResult{
			{
				Path:        "/test1",
				Method:      "GET",
				IsIdentical: true,
			},
			{
				Path:        "/test2",
				Method:      "POST",
				IsIdentical: false,
				Differences: []Difference{
					{Type: "status", Description: "Status mismatch"},
				},
			},
			{
				Path:        "/test3",
				Method:      "GET",
				IsIdentical: false,
				Differences: []Difference{
					{Type: "header", Description: "Header mismatch"},
				},
			},
		}

		for _, result := range results {
			recorder.Record(&result)
		}

		// Verify all results are recorded
		assert.Len(t, recorder.GetResults(), 3)

		// Verify failures
		failures := recorder.GetFailures()
		assert.Len(t, failures, 2)
		assert.Equal(t, "/test2", failures[0].Path)
		assert.Equal(t, "/test3", failures[1].Path)

		// Verify summary
		summary := recorder.GetSummary()
		assert.Equal(t, 3, summary.TotalRequests)
		assert.Equal(t, 1, summary.IdenticalCount)
		assert.Equal(t, 2, summary.DifferentCount)
		assert.Equal(t, 1, summary.DifferenceTypes["status"])
		assert.Equal(t, 1, summary.DifferenceTypes["header"])
	})

	t.Run("summary string format", func(t *testing.T) {
		recorder := NewComparisonRecorder()

		// Add mixed results
		for i := 0; i < 8; i++ {
			result := ComparisonResult{
				Path:        "/test",
				Method:      "GET",
				IsIdentical: i < 6, // 6 identical, 2 different
			}
			if !result.IsIdentical {
				result.Differences = []Difference{
					{Type: "status", Description: "Status mismatch"},
				}
			}
			recorder.Record(&result)
		}

		summary := recorder.GetSummary()
		summaryStr := summary.String()

		// Verify summary contains expected information
		assert.Contains(t, summaryStr, "A/B Test Summary:")
		assert.Contains(t, summaryStr, "Total Requests: 8")
		assert.Contains(t, summaryStr, "Identical: 6 (75.0%)")
		assert.Contains(t, summaryStr, "Different: 2")
		assert.Contains(t, summaryStr, "status: 2")
	})
}

func TestHeaderNormalization(t *testing.T) {
	t.Run("ignores date and request-id headers", func(t *testing.T) {
		headers1 := http.Header{
			"Date":         []string{"Mon, 01 Jan 2024 00:00:00 GMT"},
			"X-Request-Id": []string{"123-456-789"},
			"Content-Type": []string{"application/json"},
		}

		headers2 := http.Header{
			"Date":         []string{"Mon, 01 Jan 2024 00:00:01 GMT"},
			"X-Request-Id": []string{"987-654-321"},
			"Content-Type": []string{"application/json"},
		}

		norm1 := normalizeHeaders(headers1)
		norm2 := normalizeHeaders(headers2)

		// Date and X-Request-Id should be ignored
		assert.Equal(t, norm1, norm2)
		assert.Len(t, norm1, 1)
		assert.Contains(t, norm1, "content-type")
	})

	t.Run("normalizes header case", func(t *testing.T) {
		headers := http.Header{
			"Content-Type":   []string{"application/json"},
			"CONTENT-LENGTH": []string{"100"}, // Will be ignored
			"X-Custom-Header": []string{"value"},
		}

		normalized := normalizeHeaders(headers)

		// All keys should be lowercase
		assert.Contains(t, normalized, "content-type")
		assert.Contains(t, normalized, "x-custom-header")
		assert.NotContains(t, normalized, "content-length") // Ignored
	})

	t.Run("sorts header values", func(t *testing.T) {
		headers := http.Header{
			"Accept": []string{"text/plain", "application/json", "text/html"},
		}

		normalized := normalizeHeaders(headers)

		// Values should be sorted
		assert.Equal(t, []string{"application/json", "text/html", "text/plain"}, 
			normalized["accept"])
	})
}

func TestJSONComparison(t *testing.T) {
	t.Run("identical JSON objects", func(t *testing.T) {
		json1 := []byte(`{"name": "test", "value": 123, "nested": {"key": "value"}}`)
		json2 := []byte(`{"value": 123, "name": "test", "nested": {"key": "value"}}`)

		diffs := compareJSON(json1, json2)
		assert.Empty(t, diffs)
	})

	t.Run("different JSON objects", func(t *testing.T) {
		json1 := []byte(`{"name": "test1", "value": 123}`)
		json2 := []byte(`{"name": "test2", "value": 123}`)

		diffs := compareJSON(json1, json2)
		assert.Len(t, diffs, 1)
		assert.Equal(t, "body", diffs[0].Type)
		assert.Equal(t, "json", diffs[0].Field)
	})

	t.Run("non-JSON returns nil", func(t *testing.T) {
		text1 := []byte("plain text")
		text2 := []byte("plain text")

		diffs := compareJSON(text1, text2)
		assert.Nil(t, diffs)
	})

	t.Run("JSON arrays", func(t *testing.T) {
		json1 := []byte(`[1, 2, 3]`)
		json2 := []byte(`[1, 2, 3]`)

		diffs := compareJSON(json1, json2)
		assert.Empty(t, diffs)

		json3 := []byte(`[1, 2, 3]`)
		json4 := []byte(`[3, 2, 1]`)

		diffs = compareJSON(json3, json4)
		assert.Len(t, diffs, 1)
	})
}
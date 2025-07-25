// Package abtest provides A/B testing functionality for comparing Echo and Gortex routes
package abtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
)

// DualExecutor runs requests through both Echo and Gortex routes for comparison
type DualExecutor struct {
	echoApp   *echo.Echo
	gortexApp *echo.Echo // Gortex currently uses Echo under the hood
	recorder  *ComparisonRecorder
}

// NewDualExecutor creates a new dual mode test executor
func NewDualExecutor(echoApp, gortexApp *echo.Echo) *DualExecutor {
	return &DualExecutor{
		echoApp:   echoApp,
		gortexApp: gortexApp,
		recorder:  NewComparisonRecorder(),
	}
}

// ComparisonResult holds the comparison result between Echo and Gortex responses
type ComparisonResult struct {
	Path           string
	Method         string
	RequestHeaders http.Header
	RequestBody    []byte
	
	EchoResponse   *Response
	GortexResponse *Response
	
	Differences    []Difference
	IsIdentical    bool
}

// Response captures HTTP response details
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Error      error
}

// Difference describes a difference between two responses
type Difference struct {
	Type        string // "status", "header", "body"
	Field       string // specific field that differs
	EchoValue   interface{}
	GortexValue interface{}
	Description string
}

// ComparisonRecorder records all comparison results
type ComparisonRecorder struct {
	results []ComparisonResult
}

// NewComparisonRecorder creates a new recorder
func NewComparisonRecorder() *ComparisonRecorder {
	return &ComparisonRecorder{
		results: make([]ComparisonResult, 0),
	}
}

// Execute runs a request through both Echo and Gortex and compares the results
func (de *DualExecutor) Execute(req *http.Request) (*ComparisonResult, error) {
	// Clone the request for both executions
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	
	// Create result
	result := &ComparisonResult{
		Path:           req.URL.Path,
		Method:         req.Method,
		RequestHeaders: req.Header.Clone(),
		RequestBody:    bodyBytes,
		Differences:    make([]Difference, 0),
		IsIdentical:    true,
	}
	
	// Execute on Echo
	echoReq := cloneRequest(req, bodyBytes)
	echoResp := httptest.NewRecorder()
	de.echoApp.ServeHTTP(echoResp, echoReq)
	result.EchoResponse = captureResponse(echoResp)
	
	// Execute on Gortex
	gortexReq := cloneRequest(req, bodyBytes)
	gortexResp := httptest.NewRecorder()
	de.gortexApp.ServeHTTP(gortexResp, gortexReq)
	result.GortexResponse = captureResponse(gortexResp)
	
	// Compare responses
	result.Differences = de.compareResponses(result.EchoResponse, result.GortexResponse)
	result.IsIdentical = len(result.Differences) == 0
	
	// Record result
	de.recorder.Record(result)
	
	return result, nil
}

// ExecuteRequest is a convenience method that creates a request and executes it
func (de *DualExecutor) ExecuteRequest(method, path string, body io.Reader) (*ComparisonResult, error) {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, body)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	return de.Execute(req)
}

// GetRecorder returns the comparison recorder
func (de *DualExecutor) GetRecorder() *ComparisonRecorder {
	return de.recorder
}

// compareResponses compares two responses and returns differences
func (de *DualExecutor) compareResponses(echo, gortex *Response) []Difference {
	var diffs []Difference
	
	// Compare status codes
	if echo.StatusCode != gortex.StatusCode {
		diffs = append(diffs, Difference{
			Type:        "status",
			Field:       "StatusCode",
			EchoValue:   echo.StatusCode,
			GortexValue: gortex.StatusCode,
			Description: fmt.Sprintf("Status code mismatch: Echo=%d, Gortex=%d", echo.StatusCode, gortex.StatusCode),
		})
	}
	
	// Compare headers
	headerDiffs := compareHeaders(echo.Headers, gortex.Headers)
	diffs = append(diffs, headerDiffs...)
	
	// Compare body
	bodyDiffs := compareBody(echo.Body, gortex.Body)
	diffs = append(diffs, bodyDiffs...)
	
	return diffs
}

// compareHeaders compares HTTP headers
func compareHeaders(echo, gortex http.Header) []Difference {
	var diffs []Difference
	
	// Normalize headers for comparison
	echoNorm := normalizeHeaders(echo)
	gortexNorm := normalizeHeaders(gortex)
	
	// Check for missing headers in Gortex
	for key, echoValues := range echoNorm {
		if gortexValues, exists := gortexNorm[key]; !exists {
			diffs = append(diffs, Difference{
				Type:        "header",
				Field:       key,
				EchoValue:   echoValues,
				GortexValue: nil,
				Description: fmt.Sprintf("Header missing in Gortex: %s", key),
			})
		} else if !reflect.DeepEqual(echoValues, gortexValues) {
			diffs = append(diffs, Difference{
				Type:        "header",
				Field:       key,
				EchoValue:   echoValues,
				GortexValue: gortexValues,
				Description: fmt.Sprintf("Header value mismatch for %s", key),
			})
		}
	}
	
	// Check for extra headers in Gortex
	for key, gortexValues := range gortexNorm {
		if _, exists := echoNorm[key]; !exists {
			diffs = append(diffs, Difference{
				Type:        "header",
				Field:       key,
				EchoValue:   nil,
				GortexValue: gortexValues,
				Description: fmt.Sprintf("Extra header in Gortex: %s", key),
			})
		}
	}
	
	return diffs
}

// normalizeHeaders normalizes headers for comparison
func normalizeHeaders(headers http.Header) map[string][]string {
	normalized := make(map[string][]string)
	
	// Headers to ignore in comparison
	ignoreHeaders := map[string]bool{
		"date":           true,
		"x-request-id":   true, // May differ between requests
		"content-length": true, // Will be recalculated
	}
	
	for key, values := range headers {
		normalizedKey := strings.ToLower(key)
		if ignoreHeaders[normalizedKey] {
			continue
		}
		
		// Sort values for consistent comparison
		sortedValues := make([]string, len(values))
		copy(sortedValues, values)
		sort.Strings(sortedValues)
		
		normalized[normalizedKey] = sortedValues
	}
	
	return normalized
}

// compareBody compares response bodies
func compareBody(echo, gortex []byte) []Difference {
	var diffs []Difference
	
	// Try JSON comparison first
	if jsonDiffs := compareJSON(echo, gortex); jsonDiffs != nil {
		return jsonDiffs
	}
	
	// Fall back to string comparison
	if !bytes.Equal(echo, gortex) {
		diffs = append(diffs, Difference{
			Type:        "body",
			Field:       "content",
			EchoValue:   string(echo),
			GortexValue: string(gortex),
			Description: "Body content mismatch",
		})
	}
	
	return diffs
}

// compareJSON attempts to compare bodies as JSON
func compareJSON(echo, gortex []byte) []Difference {
	var echoJSON, gortexJSON interface{}
	
	// Try to parse both as JSON
	if err := json.Unmarshal(echo, &echoJSON); err != nil {
		return nil // Not JSON
	}
	if err := json.Unmarshal(gortex, &gortexJSON); err != nil {
		return nil // Not JSON
	}
	
	// Deep equal comparison
	if reflect.DeepEqual(echoJSON, gortexJSON) {
		return []Difference{}
	}
	
	// If not equal, return a difference
	return []Difference{{
		Type:        "body",
		Field:       "json",
		EchoValue:   echoJSON,
		GortexValue: gortexJSON,
		Description: "JSON body mismatch",
	}}
}

// cloneRequest creates a copy of the request with a new body
func cloneRequest(req *http.Request, body []byte) *http.Request {
	newReq := req.Clone(req.Context())
	if body != nil {
		newReq.Body = io.NopCloser(bytes.NewReader(body))
	}
	return newReq
}

// captureResponse captures response details from ResponseRecorder
func captureResponse(rec *httptest.ResponseRecorder) *Response {
	return &Response{
		StatusCode: rec.Code,
		Headers:    rec.Header().Clone(),
		Body:       rec.Body.Bytes(),
		Error:      nil,
	}
}

// Record adds a comparison result
func (cr *ComparisonRecorder) Record(result *ComparisonResult) {
	cr.results = append(cr.results, *result)
}

// GetResults returns all recorded results
func (cr *ComparisonRecorder) GetResults() []ComparisonResult {
	return cr.results
}

// GetFailures returns only results with differences
func (cr *ComparisonRecorder) GetFailures() []ComparisonResult {
	var failures []ComparisonResult
	for _, result := range cr.results {
		if !result.IsIdentical {
			failures = append(failures, result)
		}
	}
	return failures
}

// GetSummary returns a summary of all comparisons
func (cr *ComparisonRecorder) GetSummary() Summary {
	summary := Summary{
		TotalRequests:   len(cr.results),
		IdenticalCount:  0,
		DifferentCount:  0,
		DifferenceTypes: make(map[string]int),
	}
	
	for _, result := range cr.results {
		if result.IsIdentical {
			summary.IdenticalCount++
		} else {
			summary.DifferentCount++
			for _, diff := range result.Differences {
				summary.DifferenceTypes[diff.Type]++
			}
		}
	}
	
	return summary
}

// Summary provides a summary of comparison results
type Summary struct {
	TotalRequests   int
	IdenticalCount  int
	DifferentCount  int
	DifferenceTypes map[string]int // Count by difference type
}

// String returns a string representation of the summary
func (s Summary) String() string {
	successRate := float64(s.IdenticalCount) / float64(s.TotalRequests) * 100
	
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("A/B Test Summary:\n"))
	sb.WriteString(fmt.Sprintf("Total Requests: %d\n", s.TotalRequests))
	sb.WriteString(fmt.Sprintf("Identical: %d (%.1f%%)\n", s.IdenticalCount, successRate))
	sb.WriteString(fmt.Sprintf("Different: %d\n", s.DifferentCount))
	
	if len(s.DifferenceTypes) > 0 {
		sb.WriteString("\nDifference Types:\n")
		for diffType, count := range s.DifferenceTypes {
			sb.WriteString(fmt.Sprintf("  - %s: %d\n", diffType, count))
		}
	}
	
	return sb.String()
}
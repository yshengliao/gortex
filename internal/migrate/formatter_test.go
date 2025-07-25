package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatter(t *testing.T) {
	// Create sample report
	report := &Report{
		ProjectPath:     "/test/project",
		TotalFiles:      10,
		AnalyzedFiles:   5,
		MigrationEffort: "medium",
		EchoHandlers: []HandlerInfo{
			{
				File:         "handlers/user.go",
				Line:         25,
				FunctionName: "GetUser",
				ReceiverType: "UserHandler",
				Complexity:   "simple",
				Issues:       []string{},
			},
			{
				File:         "handlers/user.go",
				Line:         40,
				FunctionName: "CreateUser",
				ReceiverType: "UserHandler",
				Complexity:   "complex",
				Issues:       []string{"Uses c.Bind() - consider parameter binding"},
			},
			{
				File:         "main.go",
				Line:         15,
				FunctionName: "handleHome",
				ReceiverType: "",
				Complexity:   "simple",
				Issues:       []string{},
			},
		},
		Suggestions: []Suggestion{
			{
				Type:        "structure",
				Description: "Convert UserHandler to a service with business logic methods",
				Example:     "Example code here",
			},
			{
				Type:        "middleware",
				Description: "Review middleware usage and convert to Gortex middleware pattern",
			},
		},
	}

	t.Run("format console output", func(t *testing.T) {
		formatter := NewFormatter(false)
		output, err := formatter.Format(report, "console")
		require.NoError(t, err)

		// Verify key sections are present
		assert.Contains(t, output, "Gortex Migration Analysis Report")
		assert.Contains(t, output, "Project Summary")
		assert.Contains(t, output, "Echo Handlers Found")
		assert.Contains(t, output, "Migration Suggestions")
		assert.Contains(t, output, "Recommended Migration Steps")

		// Verify counts
		assert.Contains(t, output, "Total Go Files:   10")
		assert.Contains(t, output, "Echo Handlers:    3")
		assert.Contains(t, output, "Migration Effort: 🟡 Medium")

		// Verify handlers are listed
		assert.Contains(t, output, "GetUser")
		assert.Contains(t, output, "CreateUser")
		assert.Contains(t, output, "handleHome")

		// Verify issues are shown (but not detailed in non-verbose)
		assert.Contains(t, output, "1 issue(s)")
	})

	t.Run("format verbose console output", func(t *testing.T) {
		formatter := NewFormatter(true)
		output, err := formatter.Format(report, "console")
		require.NoError(t, err)

		// In verbose mode, issues should be detailed
		assert.Contains(t, output, "Uses c.Bind() - consider parameter binding")
		
		// Examples should be shown
		assert.Contains(t, output, "Example code here")
	})

	t.Run("format JSON output", func(t *testing.T) {
		formatter := NewFormatter(false)
		output, err := formatter.Format(report, "json")
		require.NoError(t, err)

		// Verify valid JSON
		var decoded Report
		err = json.Unmarshal([]byte(output), &decoded)
		require.NoError(t, err)

		// Verify content
		assert.Equal(t, report.ProjectPath, decoded.ProjectPath)
		assert.Equal(t, len(report.EchoHandlers), len(decoded.EchoHandlers))
		assert.Equal(t, report.MigrationEffort, decoded.MigrationEffort)
	})

	t.Run("format to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputFile := filepath.Join(tmpDir, "report.txt")

		formatter := NewFormatter(false)
		_, err := formatter.Format(report, outputFile)
		require.NoError(t, err)

		// Verify file was created
		content, err := os.ReadFile(outputFile)
		require.NoError(t, err)

		// Verify content
		assert.Contains(t, string(content), "Gortex Migration Analysis Report")
		assert.Contains(t, string(content), "Echo Handlers Found")
	})

	t.Run("migration steps based on effort", func(t *testing.T) {
		formatter := NewFormatter(false)

		testCases := []struct {
			effort string
			expect []string
		}{
			{
				effort: "none",
				expect: []string{"No migration needed"},
			},
			{
				effort: "low",
				expect: []string{
					"Install Gortex tools",
					"Create service layer",
					"Use gortex-gen",
					"Test using A/B testing",
				},
			},
			{
				effort: "medium",
				expect: []string{
					"Group related handlers",
					"Convert middleware",
					"Gradually migrate",
				},
			},
			{
				effort: "high",
				expect: []string{
					"Plan phased migration",
					"Start with simple handlers",
					"Create abstraction layer",
					"Use dual-mode operation",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.effort, func(t *testing.T) {
				testReport := *report // Copy
				testReport.MigrationEffort = tc.effort

				output, err := formatter.Format(&testReport, "console")
				require.NoError(t, err)

				for _, expected := range tc.expect {
					assert.Contains(t, output, expected)
				}
			})
		}
	})

	t.Run("effort formatting", func(t *testing.T) {
		formatter := NewFormatter(false)

		testCases := []struct {
			effort   string
			expected string
		}{
			{"none", "✅ None"},
			{"low", "🟢 Low"},
			{"medium", "🟡 Medium"},
			{"high", "🔴 High"},
		}

		for _, tc := range testCases {
			result := formatter.formatEffort(tc.effort)
			assert.Contains(t, result, tc.expected)
		}
	})
}

func TestFormatterEdgeCases(t *testing.T) {
	t.Run("empty report", func(t *testing.T) {
		report := &Report{
			ProjectPath:     "/empty",
			TotalFiles:      0,
			AnalyzedFiles:   0,
			EchoHandlers:    []HandlerInfo{},
			Suggestions:     []Suggestion{},
			MigrationEffort: "none",
		}

		formatter := NewFormatter(false)
		output, err := formatter.Format(report, "console")
		require.NoError(t, err)

		assert.Contains(t, output, "No migration needed")
		assert.NotContains(t, output, "Echo Handlers Found")
	})

	t.Run("handlers without receiver", func(t *testing.T) {
		report := &Report{
			ProjectPath: "/test",
			EchoHandlers: []HandlerInfo{
				{
					File:         "main.go",
					Line:         10,
					FunctionName: "handleFunc",
					ReceiverType: "", // No receiver
					Complexity:   "simple",
				},
			},
		}

		formatter := NewFormatter(false)
		output, err := formatter.Format(report, "console")
		require.NoError(t, err)

		// Should show dash for missing receiver
		lines := strings.Split(output, "\n")
		var foundHandler bool
		for _, line := range lines {
			if strings.Contains(line, "handleFunc") && strings.Contains(line, "-") {
				foundHandler = true
				break
			}
		}
		assert.True(t, foundHandler)
	})
}
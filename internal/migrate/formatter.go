package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// Formatter formats migration reports
type Formatter struct {
	verbose bool
}

// NewFormatter creates a new report formatter
func NewFormatter(verbose bool) *Formatter {
	return &Formatter{
		verbose: verbose,
	}
}

// Format formats the report in the specified format
func (f *Formatter) Format(report *Report, output string) (string, error) {
	switch output {
	case "console":
		return f.formatConsole(report), nil
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		// Assume it's a filename
		formatted := f.formatConsole(report)
		if err := os.WriteFile(output, []byte(formatted), 0644); err != nil {
			return "", err
		}
		return formatted, nil
	}
}

// formatConsole formats the report for console output
func (f *Formatter) formatConsole(report *Report) string {
	var buf bytes.Buffer

	// Header
	buf.WriteString("\n")
	buf.WriteString("═══════════════════════════════════════════════════════════════════\n")
	buf.WriteString("                    Gortex Migration Analysis Report                \n")
	buf.WriteString("═══════════════════════════════════════════════════════════════════\n\n")

	// Summary
	buf.WriteString("📊 Project Summary\n")
	buf.WriteString("─────────────────\n")
	fmt.Fprintf(&buf, "  Project Path:     %s\n", report.ProjectPath)
	fmt.Fprintf(&buf, "  Total Go Files:   %d\n", report.TotalFiles)
	fmt.Fprintf(&buf, "  Analyzed Files:   %d\n", report.AnalyzedFiles)
	fmt.Fprintf(&buf, "  Echo Handlers:    %d\n", len(report.EchoHandlers))
	fmt.Fprintf(&buf, "  Migration Effort: %s\n\n", f.formatEffort(report.MigrationEffort))

	// Echo Handlers
	if len(report.EchoHandlers) > 0 {
		buf.WriteString("🔍 Echo Handlers Found\n")
		buf.WriteString("─────────────────────\n")
		
		// Group by file
		handlersByFile := make(map[string][]HandlerInfo)
		for _, handler := range report.EchoHandlers {
			handlersByFile[handler.File] = append(handlersByFile[handler.File], handler)
		}

		for file, handlers := range handlersByFile {
			fmt.Fprintf(&buf, "\n📄 %s\n", file)
			
			w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  Line\tFunction\tReceiver\tComplexity\tIssues")
			fmt.Fprintln(w, "  ────\t────────\t────────\t──────────\t──────")
			
			for _, h := range handlers {
				issues := "None"
				if len(h.Issues) > 0 {
					if f.verbose {
						issues = strings.Join(h.Issues, "; ")
					} else {
						issues = fmt.Sprintf("%d issue(s)", len(h.Issues))
					}
				}
				
				receiver := h.ReceiverType
				if receiver == "" {
					receiver = "-"
				}
				
				fmt.Fprintf(w, "  %d\t%s\t%s\t%s\t%s\n",
					h.Line, h.FunctionName, receiver, 
					f.formatComplexity(h.Complexity), issues)
			}
			w.Flush()

			// Show detailed issues in verbose mode
			if f.verbose {
				for _, h := range handlers {
					if len(h.Issues) > 0 {
						fmt.Fprintf(&buf, "\n    Details for %s (line %d):\n", h.FunctionName, h.Line)
						for _, issue := range h.Issues {
							fmt.Fprintf(&buf, "      • %s\n", issue)
						}
					}
				}
			}
		}
		buf.WriteString("\n")
	}

	// Migration Suggestions
	if len(report.Suggestions) > 0 {
		buf.WriteString("💡 Migration Suggestions\n")
		buf.WriteString("──────────────────────\n")
		
		// Group by type
		suggestionsByType := make(map[string][]Suggestion)
		for _, s := range report.Suggestions {
			suggestionsByType[s.Type] = append(suggestionsByType[s.Type], s)
		}

		typeOrder := []string{"structure", "handler", "middleware"}
		typeIcons := map[string]string{
			"structure":  "🏗️",
			"handler":    "🔧",
			"middleware": "🔌",
		}

		for _, sType := range typeOrder {
			suggestions, exists := suggestionsByType[sType]
			if !exists {
				continue
			}

			icon := typeIcons[sType]
			fmt.Fprintf(&buf, "\n%s %s Suggestions:\n", icon, strings.Title(sType))
			
			for i, s := range suggestions {
				fmt.Fprintf(&buf, "\n  %d. %s\n", i+1, s.Description)
				
				if s.File != "" {
					fmt.Fprintf(&buf, "     File: %s", s.File)
					if s.Line > 0 {
						fmt.Fprintf(&buf, ":%d", s.Line)
					}
					buf.WriteString("\n")
				}
				
				if f.verbose && s.Example != "" {
					buf.WriteString("\n     Example:\n")
					for _, line := range strings.Split(s.Example, "\n") {
						fmt.Fprintf(&buf, "     %s\n", line)
					}
				}
			}
		}
		buf.WriteString("\n")
	}

	// Migration Steps
	buf.WriteString("📋 Recommended Migration Steps\n")
	buf.WriteString("────────────────────────────\n")
	buf.WriteString(f.getMigrationSteps(report))
	buf.WriteString("\n")

	// Resources
	buf.WriteString("📚 Resources\n")
	buf.WriteString("───────────\n")
	buf.WriteString("  • Gortex Documentation: https://github.com/yshengliao/gortex\n")
	buf.WriteString("  • Migration Guide: https://github.com/yshengliao/gortex/docs/migration.md\n")
	buf.WriteString("  • Code Generation: Use 'gortex-gen' for automatic handler generation\n")
	buf.WriteString("  • A/B Testing: Use 'gortex-cli test' to verify migration correctness\n\n")

	return buf.String()
}

// formatEffort formats the migration effort with color/emoji
func (f *Formatter) formatEffort(effort string) string {
	switch effort {
	case "none":
		return "✅ None (No Echo handlers found)"
	case "low":
		return "🟢 Low"
	case "medium":
		return "🟡 Medium"
	case "high":
		return "🔴 High"
	default:
		return effort
	}
}

// formatComplexity formats handler complexity
func (f *Formatter) formatComplexity(complexity string) string {
	switch complexity {
	case "simple":
		return "Simple"
	case "medium":
		return "Medium"
	case "complex":
		return "Complex"
	default:
		return complexity
	}
}

// getMigrationSteps provides step-by-step migration guidance
func (f *Formatter) getMigrationSteps(report *Report) string {
	var steps []string

	if report.MigrationEffort == "none" {
		return "  No migration needed - project doesn't use Echo handlers.\n"
	}

	// Basic steps
	steps = append(steps, "1. Install Gortex tools: go install github.com/yshengliao/gortex/cmd/gortex-gen@latest")
	steps = append(steps, "2. Create service layer by extracting business logic from handlers")
	
	// Effort-specific steps
	switch report.MigrationEffort {
	case "low":
		steps = append(steps, "3. Use gortex-gen to generate handlers from service methods")
		steps = append(steps, "4. Update routing to use Gortex declarative style")
		steps = append(steps, "5. Test using A/B testing framework")
	case "medium":
		steps = append(steps, "3. Group related handlers into service structs")
		steps = append(steps, "4. Convert middleware to Gortex pattern")
		steps = append(steps, "5. Use gortex-gen for handler generation")
		steps = append(steps, "6. Gradually migrate handlers using compatibility mode")
		steps = append(steps, "7. Validate with A/B testing at each step")
	case "high":
		steps = append(steps, "3. Plan phased migration approach")
		steps = append(steps, "4. Start with simple handlers as proof of concept")
		steps = append(steps, "5. Create abstraction layer for complex handlers")
		steps = append(steps, "6. Migrate in batches with thorough testing")
		steps = append(steps, "7. Use dual-mode operation during transition")
	}

	// Format steps
	var buf bytes.Buffer
	for _, step := range steps {
		fmt.Fprintf(&buf, "  %s\n", step)
	}

	return buf.String()
}
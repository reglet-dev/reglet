package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/owenrumney/go-sarif/v3/pkg/report/v210/sarif"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

type sarifMapper struct {
	result      *execution.ExecutionResult
	profilePath string
	cwd         string                     // Current working directory
	artifacts   map[string]*sarif.Artifact // Deduplicated artifacts
}

func newSARIFMapper(result *execution.ExecutionResult, profilePath string) *sarifMapper {
	cwd, _ := os.Getwd() // Best effort, ignore error
	return &sarifMapper{
		result:      result,
		profilePath: profilePath,
		cwd:         cwd,
		artifacts:   make(map[string]*sarif.Artifact),
	}
}

// mapToRun populates the SARIF run with rules, results, artifacts, and invocations.
func (m *sarifMapper) mapToRun(run *sarif.Run) {
	m.addRules(run)
	m.addResults(run)
	m.addArtifacts(run)
	m.addInvocation(run)
	m.addProperties(run)
}

// addRules converts controls to SARIF rules.
func (m *sarifMapper) addRules(run *sarif.Run) {
	for _, ctrl := range m.result.Controls {
		rule := sarif.NewReportingDescriptor().WithID(ctrl.ID)

		rule.WithName(ctrl.Name)

		// Short description
		rule.WithShortDescription(&sarif.MultiformatMessageString{
			Text: &ctrl.Name,
		})

		// Full description (fallback to name if empty)
		desc := ctrl.Description
		if desc == "" {
			desc = ctrl.Name
		}
		rule.WithFullDescription(&sarif.MultiformatMessageString{
			Text: &desc,
		})

		// Default configuration (severity → level)
		level := m.mapSeverityToLevel(ctrl.Severity)
		rule.WithDefaultConfiguration(&sarif.ReportingConfiguration{
			Level: level,
		})

		// Properties (tags, severity)
		props := sarif.NewPropertyBag()
		if len(ctrl.Tags) > 0 {
			props.WithTags(ctrl.Tags)
		}
		if ctrl.Severity != "" {
			props.Add("severity", ctrl.Severity)
		}
		rule.WithProperties(props)

		run.Tool.Driver.AddRule(rule)
	}
}

// addResults converts control results to SARIF results.
func (m *sarifMapper) addResults(run *sarif.Run) {
	for _, ctrl := range m.result.Controls {
		result := m.mapControlResult(ctrl)
		run.AddResult(result)
	}
}

// mapControlResult converts a single ControlResult to a SARIF Result.
func (m *sarifMapper) mapControlResult(ctrl execution.ControlResult) *sarif.Result {
	result := sarif.NewRuleResult(ctrl.ID)

	// Set level and kind based on status + severity
	result.Level = m.mapStatusToLevel(ctrl.Status, ctrl.Severity)
	result.Kind = m.mapStatusToKind(ctrl.Status)

	// Set message (use ctrl.Message or generate default)
	msg := ctrl.Message
	if msg == "" {
		msg = m.generateDefaultMessage(ctrl)
	}
	result.Message = sarif.NewTextMessage(msg)

	// Extract location from evidence
	if loc := m.extractLocation(ctrl); loc != nil {
		result.Locations = []*sarif.Location{loc}
	}

	// Add properties (observations, duration, metadata)
	props := sarif.NewPropertyBag()
	props.Add("observations", ctrl.ObservationResults)
	props.Add("duration_ms", ctrl.Duration.Milliseconds())

	if len(ctrl.Tags) > 0 {
		props.WithTags(ctrl.Tags)
	}
	if ctrl.Severity != "" {
		props.Add("severity", ctrl.Severity)
	}
	if ctrl.SkipReason != "" {
		props.Add("skipReason", ctrl.SkipReason)
	}
	result.WithProperties(props)

	return result
}

// mapStatusToLevel converts Reglet status + severity to SARIF level.
func (m *sarifMapper) mapStatusToLevel(status values.Status, severity string) string {
	switch status {
	case values.StatusPass:
		return "note"
	case values.StatusFail:
		// Use severity to determine error vs warning
		switch severity {
		case "critical", "high":
			return "error"
		case "medium", "low":
			return "warning"
		default:
			return "warning"
		}
	case values.StatusError:
		return "error"
	case values.StatusSkipped:
		return "none"
	default:
		return "warning"
	}
}

// mapStatusToKind converts Reglet status to SARIF kind.
func (m *sarifMapper) mapStatusToKind(status values.Status) string {
	switch status {
	case values.StatusPass:
		return "pass"
	case values.StatusFail, values.StatusError:
		return "fail"
	case values.StatusSkipped:
		return "notApplicable"
	default:
		return "fail"
	}
}

// mapSeverityToLevel converts severity alone to SARIF level (for rule default).
func (m *sarifMapper) mapSeverityToLevel(severity string) string {
	switch severity {
	case "critical", "high":
		return "error"
	case "medium", "low":
		return "warning"
	default:
		return "warning"
	}
}

// extractLocation attempts to extract file location from observations.
func (m *sarifMapper) extractLocation(ctrl execution.ControlResult) *sarif.Location {
	for _, obs := range ctrl.ObservationResults {
		if obs.Evidence == nil || obs.Evidence.Data == nil {
			continue
		}

		data := obs.Evidence.Data

		// Check for file path (file plugin)
		if pathVal, ok := data["path"]; ok {
			if path, ok := pathVal.(string); ok && path != "" {
				return m.createLocation(path, data)
			}
		}

		// Check for command path (command plugin with "command" config)
		if cmdPathVal, ok := data["command_path"]; ok {
			if cmdPath, ok := cmdPathVal.(string); ok && cmdPath != "" {
				return m.createLocation(cmdPath, data)
			}
		}

		// Check for shell command (command plugin with "run" config)
		if shellCmdVal, ok := data["shell_command"]; ok {
			if shellCmd, ok := shellCmdVal.(string); ok && shellCmd != "" {
				// Only use as location if it's not an inline command
				if !strings.ContainsAny(shellCmd, " ;\n") && len(shellCmd) < 256 {
					return m.createLocation(shellCmd, data)
				}
			}
		}
	}

	// No location found (valid for network checks)
	return nil
}

func (m *sarifMapper) createLocation(path string, data map[string]interface{}) *sarif.Location {
	uri := m.normalizeURI(path)

	// Register artifact for run.artifacts
	m.registerArtifact(path, data)

	pLoc := sarif.NewPhysicalLocation().
		WithArtifactLocation(sarif.NewArtifactLocation().WithURI(uri))

	// Try to extract line/column
	if line := m.getInt(data, "line", "start_line", "lineNumber"); line > 0 {
		region := sarif.NewRegion().WithStartLine(line)

		if col := m.getInt(data, "column", "start_column", "columnNumber"); col > 0 {
			region.WithStartColumn(col)
		}
		pLoc.WithRegion(region)
	}

	return sarif.NewLocation().WithPhysicalLocation(pLoc)
}

// normalizeURI converts a file path to a SARIF-compliant URI.
func (m *sarifMapper) normalizeURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path) // Fallback to original
	}

	// Try to make relative to CWD
	if m.cwd != "" {
		if rel, err := filepath.Rel(m.cwd, abs); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}

	// Use absolute file:// URI
	return "file://" + filepath.ToSlash(abs)
}

// getInt extracts an integer from map[string]interface{}.
func (m *sarifMapper) getInt(data map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			}
		}
	}
	return 0
}

// registerArtifact adds a file to the artifacts map (deduplicated).
// registerArtifact adds a file to the artifacts map (deduplicated).
func (m *sarifMapper) registerArtifact(path string, data map[string]interface{}) {
	uri := m.normalizeURI(path)
	if _, exists := m.artifacts[uri]; exists {
		return // Already registered
	}

	artifact := sarif.NewArtifact().
		WithLocation(sarif.NewArtifactLocation().WithURI(uri))

	// Attempt to read file content to embed in SARIF
	// This allows viewers to show the code context
	absPath, err := filepath.Abs(path)
	if err == nil {
		// Limit content size to prevents massive SARIF files
		const maxContentSize = 512 * 1024 // 512KB limit

		// Use os.Stat to check size first
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			if info.Size() < maxContentSize {
				//nolint:gosec // G304: absPath is from evidence data, bounded by size check above
				content, err := os.ReadFile(absPath)
				if err == nil {
					artifact.WithContents(sarif.NewArtifactContent().WithText(string(content)))
				}
			}
		}
	}

	// Add properties from evidence
	props := sarif.NewPropertyBag()
	if exists, ok := data["exists"].(bool); ok {
		props.Add("exists", exists)
	}
	if readable, ok := data["readable"].(bool); ok {
		props.Add("readable", readable)
	}

	// Add file size (length)
	if size, ok := data["size"].(int); ok {
		artifact.WithLength(size)
	} else if size, ok := data["size"].(float64); ok {
		artifact.WithLength(int(size))
	}

	artifact.WithProperties(props)
	m.artifacts[uri] = artifact
}

// addArtifacts adds collected artifacts to the run.
func (m *sarifMapper) addArtifacts(run *sarif.Run) {
	for _, artifact := range m.artifacts {
		run.AddArtifact(artifact)
	}
}

// addInvocation adds execution metadata to the run.
func (m *sarifMapper) addInvocation(run *sarif.Run) {
	invocation := sarif.NewInvocation()

	// Execution success (no errors)
	invocation.ExecutionSuccessful = ptrBool(m.result.Summary.ErrorControls == 0)

	// Timestamps (UTC, ISO 8601 format)
	startTime := m.result.StartTime.UTC().Format("2006-01-02T15:04:05.000Z")
	endTime := m.result.EndTime.UTC().Format("2006-01-02T15:04:05.000Z")
	invocation.StartTimeUtc = &startTime
	invocation.EndTimeUtc = &endTime

	// Machine name
	if hostname, err := os.Hostname(); err == nil {
		invocation.Machine = &hostname
	}

	// Working directory
	if m.cwd != "" {
		cwd := "file://" + filepath.ToSlash(m.cwd)
		invocation.WorkingDirectory = sarif.NewArtifactLocation().WithURI(cwd)
	}

	// Execution metadata in properties
	props := sarif.NewPropertyBag()
	props.Add("profileName", m.result.ProfileName)
	props.Add("profileVersion", m.result.ProfileVersion)
	props.Add("executionId", m.result.ExecutionID)
	invocation.WithProperties(props)

	run.AddInvocation(invocation)
}

// addProperties adds summary statistics to run properties.
func (m *sarifMapper) addProperties(run *sarif.Run) {
	props := sarif.NewPropertyBag()
	props.Add("summary", m.result.Summary)
	run.WithProperties(props)
}

// generateDefaultMessage creates a default message for controls without one.
func (m *sarifMapper) generateDefaultMessage(ctrl execution.ControlResult) string {
	switch ctrl.Status {
	case values.StatusPass:
		return fmt.Sprintf("Control %s passed", ctrl.ID)
	case values.StatusFail:
		return fmt.Sprintf("Control %s failed", ctrl.ID)
	case values.StatusError:
		return fmt.Sprintf("Control %s encountered an error", ctrl.ID)
	case values.StatusSkipped:
		return fmt.Sprintf("Control %s was skipped", ctrl.ID)
	default:
		return fmt.Sprintf("Control %s completed with status %s", ctrl.ID, ctrl.Status)
	}
}

func ptrBool(b bool) *bool {
	return &b
}

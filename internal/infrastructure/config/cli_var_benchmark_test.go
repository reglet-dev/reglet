package config

import (
	"fmt"
	"testing"
)

// BenchmarkParseCLIVar benchmarks parsing single CLI variable.
// Covers T053: Add benchmark tests for flag parsing performance.
func BenchmarkParseCLIVar(b *testing.B) {
	inputs := []string{
		"simple=value",
		"nested.path.key=value",
		"port=8080",
		"debug=true",
		"float=3.14",
	}

	for _, input := range inputs {
		b.Run(input, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = ParseCLIVar(input)
			}
		})
	}
}

// BenchmarkParseMultipleCLIVars benchmarks parsing multiple CLI variables.
func BenchmarkParseMultipleCLIVars(b *testing.B) {
	inputs := [][]string{
		{"single=value"},
		{"a=1", "b=2", "c=3", "d=4", "e=5"},
		{"env=prod", "debug=false", "port=8080", "host=localhost", "timeout=30"},
		generateNVars(10),
		generateNVars(50),
	}

	for _, inputSet := range inputs {
		name := fmt.Sprintf("%d_vars", len(inputSet))
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = ParseMultipleCLIVars(inputSet)
			}
		})
	}
}

// BenchmarkDetectValueType benchmarks type detection.
func BenchmarkDetectValueType(b *testing.B) {
	values := []string{
		"simple string value",
		"true",
		"false",
		"12345",
		"-999",
		"3.14159",
		"-0.001",
		"1.0.0", // Version string
		"00123", // Leading zero
	}

	for _, v := range values {
		b.Run(v, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = DetectValueType(v)
			}
		})
	}
}

// BenchmarkMergeCLIVars benchmarks merging CLI vars with profile vars.
func BenchmarkMergeCLIVars(b *testing.B) {
	profileVars := map[string]interface{}{
		"env":     "dev",
		"port":    8080,
		"debug":   false,
		"timeout": 30,
		"paths": map[string]interface{}{
			"config": "/etc/app",
			"data":   "/var/data",
			"logs":   "/var/log",
		},
	}

	cliVars := map[string]interface{}{
		"env":   "prod",
		"debug": true,
		"paths": map[string]interface{}{
			"config": "/opt/app",
		},
		"new_var": "injected",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MergeCLIVars(profileVars, cliVars)
	}
}

// BenchmarkFindUnusedVars benchmarks unused variable detection.
func BenchmarkFindUnusedVars(b *testing.B) {
	profileContent := `
profile:
  name: test

vars:
  env: dev
  port: 8080

controls:
  items:
    - id: check
      observe:
        - plugin: http
          config:
            url: "http://{{ .vars.host }}:{{ .vars.port }}/health"
            timeout: "{{ .vars.timeout }}ms"
          expect:
            - data.status == 200
`
	cliVars := map[string]interface{}{
		"host":       "localhost",
		"port":       8080,
		"timeout":    1000,
		"unused_var": "value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FindUnusedVars(cliVars, profileContent)
	}
}

// BenchmarkSetNestedValue benchmarks nested path insertion.
func BenchmarkSetNestedValue(b *testing.B) {
	paths := []string{
		"simple",
		"one.two",
		"one.two.three",
		"a.b.c.d.e",
	}

	for _, path := range paths {
		b.Run(path, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				m := make(map[string]interface{})
				_ = SetNestedValue(m, path, "value")
			}
		})
	}
}

// generateNVars creates N key=value pairs for benchmarking.
func generateNVars(n int) []string {
	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = fmt.Sprintf("var%d=value%d", i, i)
	}
	return result
}

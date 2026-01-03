//go:build wasip1

package main

// AWSConfig defines the configuration structure for the AWS plugin.
type AWSConfig struct {
	Service        string                   `json:"service" validate:"required,oneof=ec2 s3 iam vpc"`
	Operation      string                   `json:"operation" validate:"required"`
	Region         string                   `json:"region,omitempty"`
	Filters        []map[string]interface{} `json:"filters,omitempty"`
	Pagination     *PaginationConfig        `json:"pagination,omitempty"`
	TimeoutSeconds int                      `json:"timeout_seconds" default:"30"`
}

type PaginationConfig struct {
	MaxResults int `json:"max_results" default:"100"`
	MaxPages   int `json:"max_pages" default:"10"`
}

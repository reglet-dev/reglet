package core

import (
	"errors"
	"strings"

	"github.com/reglet-dev/reglet-sdk/domain/entities"
)

// MapAWSErrorToSDK converts AWS errors to SDK error types.
func MapAWSErrorToSDK(err error) *entities.ErrorDetail {
	var awsErr *AWSError
	if !errors.As(err, &awsErr) {
		return entities.NewErrorDetail("network", err.Error()).WithCode("aws_error")
	}

	code := awsErr.Code
	msg := awsErr.Message

	switch {
	case code == "AccessDenied" || code == "UnauthorizedOperation":
		return &entities.ErrorDetail{
			Type:    "capability",
			Code:    code,
			Message: "AWS permission denied: " + msg,
		}

	case code == "Throttling" || code == "RequestLimitExceeded":
		return &entities.ErrorDetail{
			Type:      "timeout",
			Code:      code,
			Message:   "AWS rate limit exceeded: " + msg,
			IsTimeout: true,
		}

	case strings.HasPrefix(code, "InvalidParameter") || code == "ValidationException":
		return &entities.ErrorDetail{
			Type:    "config",
			Code:    code,
			Message: "Invalid AWS parameter: " + msg,
		}

	case code == "ResourceNotFoundException" || strings.HasSuffix(code, "NotFound"):
		return &entities.ErrorDetail{
			Type:       "config",
			Code:       code,
			Message:    "AWS resource not found: " + msg,
			IsNotFound: true,
		}

	case code == "ServiceUnavailable" || code == "InternalFailure":
		return &entities.ErrorDetail{
			Type:    "network",
			Code:    code,
			Message: "AWS service unavailable: " + msg,
		}

	default:
		return &entities.ErrorDetail{
			Type:    "internal",
			Code:    code,
			Message: "AWS API error: " + msg,
		}
	}
}

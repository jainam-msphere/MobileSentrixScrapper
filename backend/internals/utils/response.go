package utils

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

func SendJSON(ctx *fasthttp.RequestCtx, statusCode int, data interface{}) {
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(statusCode)

	if data == nil {
		return
	}

	if err := json.NewEncoder(ctx).Encode(data); err != nil {
		ctx.Error("failed to encode response", fasthttp.StatusInternalServerError)
	}
}

// SendError sends an error response following AEP-193 RFC 9457 Problem Details format
func SendError(ctx *fasthttp.RequestCtx, statusCode int, title string, detail interface{}) {
	error := map[string]interface{}{
		"type":   GetErrorType(statusCode),
		"status": statusCode,
		"title":  title,
	}

	if detail != nil {
		// If detail is a string, use it directly; otherwise add as metadata
		if detailStr, ok := detail.(string); ok {
			error["detail"] = detailStr
		} else {
			error["detail"] = title
			error["metadata"] = detail
		}
	}

	ctx.Response.Header.Set("Content-Type", "application/problem+json")
	SendJSON(ctx, statusCode, error)
}

// GetErrorType returns the error type based on HTTP status code
func GetErrorType(statusCode int) string {
	switch statusCode {
	case 400:
		return "INVALID_ARGUMENT"
	case 401:
		return "UNAUTHENTICATED"
	case 403:
		return "PERMISSION_DENIED"
	case 404:
		return "NOT_FOUND"
	case 409:
		return "ALREADY_EXISTS"
	case 429:
		return "RESOURCE_EXHAUSTED"
	case 500:
		return "INTERNAL"
	case 501:
		return "NOT_IMPLEMENTED"
	case 503:
		return "UNAVAILABLE"
	default:
		return "UNKNOWN"
	}
}

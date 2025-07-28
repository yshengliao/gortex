package fixture

import (
	"encoding/json"
	"time"
)

// SampleUser returns a sample user for testing
func SampleUser() map[string]interface{} {
	return map[string]interface{}{
		"id":       123,
		"username": "testuser",
		"email":    "test@example.com",
		"role":     "user",
		"active":   true,
		"created":  time.Now().Format(time.RFC3339),
	}
}

// SampleRequest returns sample request data
func SampleRequest() map[string]interface{} {
	return map[string]interface{}{
		"action": "create",
		"data": map[string]interface{}{
			"name":        "Test Item",
			"description": "A test item for testing",
			"price":       19.99,
			"quantity":    5,
		},
		"timestamp": time.Now().Unix(),
	}
}

// SampleJSONBody returns a JSON-encoded sample request body
func SampleJSONBody(data interface{}) []byte {
	if data == nil {
		data = SampleRequest()
	}
	body, _ := json.Marshal(data)
	return body
}

// SampleError returns a sample error response
func SampleError(code int, message string) map[string]interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}
}

// ValidationErrors returns sample validation errors
func ValidationErrors() map[string]interface{} {
	return map[string]interface{}{
		"errors": []map[string]interface{}{
			{
				"field":   "username",
				"message": "Username is required",
			},
			{
				"field":   "email",
				"message": "Invalid email format",
			},
		},
	}
}
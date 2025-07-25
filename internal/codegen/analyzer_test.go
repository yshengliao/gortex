package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseMethod(t *testing.T, code string) *ast.FuncDecl {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return fn
		}
	}
	t.Fatal("No function declaration found")
	return nil
}

func TestAnalyzeMethod(t *testing.T) {
	testCases := []struct {
		name           string
		code           string
		expectedMethod MethodInfo
	}{
		{
			name: "simple GET method",
			code: `package test
func (h *Handler) GET(c echo.Context) error {
	return nil
}`,
			expectedMethod: MethodInfo{
				Name:       "GET",
				HTTPMethod: "GET",
				HasContext: true,
				Receiver: ReceiverInfo{
					Name:  "h",
					Type:  "Handler",
					IsPtr: true,
				},
				Params: []ParamInfo{
					{Name: "c", Type: "echo.Context"},
				},
				Results: []ResultInfo{
					{Type: "error", IsError: true},
				},
			},
		},
		{
			name: "method with request struct",
			code: `package test
func (s *Service) CreateUser(c echo.Context, req *CreateUserRequest) error {
	return nil
}`,
			expectedMethod: MethodInfo{
				Name:       "CreateUser",
				HTTPMethod: "POST",
				HasContext: true,
				Receiver: ReceiverInfo{
					Name:  "s",
					Type:  "Service",
					IsPtr: true,
				},
				Params: []ParamInfo{
					{Name: "c", Type: "echo.Context"},
					{Name: "req", Type: "*CreateUserRequest", IsPtr: true, IsStruct: true},
				},
				Results: []ResultInfo{
					{Type: "error", IsError: true},
				},
			},
		},
		{
			name: "method with multiple params and results",
			code: `package test
func (s *Service) UpdateUser(c echo.Context, id int, req *UpdateRequest) (*User, error) {
	return nil, nil
}`,
			expectedMethod: MethodInfo{
				Name:       "UpdateUser",
				HTTPMethod: "POST",
				HasContext: true,
				Receiver: ReceiverInfo{
					Name:  "s",
					Type:  "Service",
					IsPtr: true,
				},
				Params: []ParamInfo{
					{Name: "c", Type: "echo.Context"},
					{Name: "id", Type: "int"},
					{Name: "req", Type: "*UpdateRequest", IsPtr: true, IsStruct: true},
				},
				Results: []ResultInfo{
					{Type: "*User"},
					{Type: "error", IsError: true},
				},
			},
		},
		{
			name: "method without echo.Context",
			code: `package test
func (s *Service) ProcessData(data []byte) (string, error) {
	return "", nil
}`,
			expectedMethod: MethodInfo{
				Name:       "ProcessData",
				HTTPMethod: "POST",
				HasContext: false,
				Receiver: ReceiverInfo{
					Name:  "s",
					Type:  "Service",
					IsPtr: true,
				},
				Params: []ParamInfo{
					{Name: "data", Type: "[]byte"},
				},
				Results: []ResultInfo{
					{Type: "string"},
					{Type: "error", IsError: true},
				},
			},
		},
		{
			name: "method with imported types",
			code: `package test
func (h *Handler) HandleRequest(c echo.Context, req *http.Request) (*http.Response, error) {
	return nil, nil
}`,
			expectedMethod: MethodInfo{
				Name:       "HandleRequest",
				HTTPMethod: "POST",
				HasContext: true,
				Receiver: ReceiverInfo{
					Name:  "h",
					Type:  "Handler",
					IsPtr: true,
				},
				Params: []ParamInfo{
					{Name: "c", Type: "echo.Context"},
					{Name: "req", Type: "*http.Request", IsPtr: true, IsStruct: true},
				},
				Results: []ResultInfo{
					{Type: "*http.Response"},
					{Type: "error", IsError: true},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn := parseMethod(t, tc.code)
			info, err := AnalyzeMethod(fn)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedMethod.Name, info.Name)
			assert.Equal(t, tc.expectedMethod.HTTPMethod, info.HTTPMethod)
			assert.Equal(t, tc.expectedMethod.HasContext, info.HasContext)
			
			// Check receiver
			assert.Equal(t, tc.expectedMethod.Receiver.Name, info.Receiver.Name)
			assert.Equal(t, tc.expectedMethod.Receiver.Type, info.Receiver.Type)
			assert.Equal(t, tc.expectedMethod.Receiver.IsPtr, info.Receiver.IsPtr)

			// Check params
			require.Len(t, info.Params, len(tc.expectedMethod.Params))
			for i, param := range tc.expectedMethod.Params {
				assert.Equal(t, param.Name, info.Params[i].Name)
				assert.Equal(t, param.Type, info.Params[i].Type)
				assert.Equal(t, param.IsPtr, info.Params[i].IsPtr)
				assert.Equal(t, param.IsStruct, info.Params[i].IsStruct)
			}

			// Check results
			require.Len(t, info.Results, len(tc.expectedMethod.Results))
			for i, result := range tc.expectedMethod.Results {
				assert.Equal(t, result.Type, info.Results[i].Type)
				assert.Equal(t, result.IsError, info.Results[i].IsError)
			}
		})
	}
}

func TestDetectHTTPMethod(t *testing.T) {
	testCases := []struct {
		methodName string
		expected   string
	}{
		{"GET", "GET"},
		{"Get", "GET"},
		{"GetUser", "GET"},
		{"GetUserByID", "GET"},
		{"POST", "POST"},
		{"CreateUser", "POST"},
		{"PUT", "PUT"},
		{"UpdateUser", "POST"}, // Update defaults to POST
		{"DELETE", "DELETE"},
		{"DeleteUser", "DELETE"},
		{"PATCH", "PATCH"},
		{"PatchUser", "PATCH"},
		{"CustomMethod", "POST"}, // Default to POST
		{"Process", "POST"},
		{"Handle", "POST"},
	}

	for _, tc := range testCases {
		t.Run(tc.methodName, func(t *testing.T) {
			result := detectHTTPMethod(tc.methodName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetRoutePattern(t *testing.T) {
	testCases := []struct {
		methodName string
		expected   string
	}{
		{"GetUser", "/user"},
		{"GetUsers", "/users"},
		{"CreateUser", "/user"},
		{"UpdateUserProfile", "/user-profile"},
		{"DeleteUserByID", "/user-by-id"},
		{"ListProducts", "/products"},
		{"GET", "/"},
		{"HandleWebhook", "/handle-webhook"},
		{"Process", "/process"},
		{"ProcessOrderItems", "/process-order-items"},
	}

	for _, tc := range testCases {
		t.Run(tc.methodName, func(t *testing.T) {
			result := GetRoutePattern(tc.methodName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsStructType(t *testing.T) {
	testCases := []struct {
		name     string
		code     string
		expected bool
	}{
		{
			name:     "pointer to struct",
			code:     `func test(u *User) {}`,
			expected: true,
		},
		{
			name:     "struct",
			code:     `func test(u User) {}`,
			expected: true,
		},
		{
			name:     "basic type",
			code:     `func test(s string) {}`,
			expected: false,
		},
		{
			name:     "slice",
			code:     `func test(s []string) {}`,
			expected: false,
		},
		{
			name:     "echo.Context",
			code:     `func test(c echo.Context) {}`,
			expected: false,
		},
		{
			name:     "imported struct",
			code:     `func test(r *http.Request) {}`,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn := parseMethod(t, "package test\n"+tc.code)
			require.NotNil(t, fn.Type.Params)
			require.Greater(t, len(fn.Type.Params.List), 0)
			
			paramType := fn.Type.Params.List[0].Type
			result := isStructType(paramType)
			assert.Equal(t, tc.expected, result)
		})
	}
}
package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHandler(t *testing.T) {
	testCases := []struct {
		name           string
		serviceCode    string
		expectedImports []string
		expectedContains []string
		notExpected    []string
	}{
		{
			name: "simple GET handler",
			serviceCode: `package service

type UserService struct{}

//go:generate gortex-gen handler
func (s *UserService) GET(c echo.Context) error {
	return nil
}`,
			expectedImports: []string{
				`"github.com/labstack/echo/v4"`,
				`"github.com/yshengliao/gortex/pkg/response"`,
				`"github.com/yshengliao/gortex/pkg/errors"`,
			},
			expectedContains: []string{
				"type UserServiceHandler struct",
				"func NewUserServiceHandler(service *service.UserService) *UserServiceHandler",
				"func (h *UserServiceHandler) GET(c echo.Context) error",
				"h.service.GET(",
				"c.Request().Context()",
			},
			notExpected: []string{
				"binder",
				"Bind(",
				"strconv",
			},
		},
		{
			name: "POST handler with request struct",
			serviceCode: `package service

type UserService struct{}

type CreateUserRequest struct {
	Name  string
	Email string
}

//go:generate gortex-gen handler
func (s *UserService) CreateUser(c echo.Context, req *CreateUserRequest) (*User, error) {
	return nil, nil
}

type User struct {
	ID string
}`,
			expectedImports: []string{
				`"github.com/labstack/echo/v4"`,
				`"github.com/yshengliao/gortex/pkg/response"`,
				`"github.com/yshengliao/gortex/app"`,
			},
			expectedContains: []string{
				"binder  *app.ParameterBinder",
				"func NewUserServiceHandler(service *service.UserService, binder *app.ParameterBinder)",
				"var req *CreateUserRequest",
				"if err := h.binder.Bind(c, &req); err != nil",
				"result0, err := h.service.CreateUser(",
				"return response.Success(c, http.StatusOK, result0)",
			},
		},
		{
			name: "handler with path parameters",
			serviceCode: `package service

type UserService struct{}

//go:generate gortex-gen handler
func (s *UserService) GetUserByID(c echo.Context, id string) (*User, error) {
	return nil, nil
}

type User struct {
	ID string
}`,
			expectedImports: []string{
				`"github.com/labstack/echo/v4"`,
			},
			expectedContains: []string{
				`id := c.Param("id")`,
				"result0, err := h.service.GetUserByID(",
				"c.Request().Context(), id)",
			},
		},
		{
			name: "handler with int path parameter",
			serviceCode: `package service

type ProductService struct{}

//go:generate gortex-gen handler
func (s *ProductService) GetProduct(c echo.Context, id int) (*Product, error) {
	return nil, nil
}

type Product struct {
	ID int
}`,
			expectedImports: []string{
				`"strconv"`,
			},
			expectedContains: []string{
				`id := c.Param("id")`,
				`idInt, err := strconv.Atoi(id)`,
				`return response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid id"`,
				"h.service.GetProduct(",
				"c.Request().Context(), idInt)",
			},
		},
		{
			name: "handler without echo.Context",
			serviceCode: `package service

type DataService struct{}

//go:generate gortex-gen handler
func (s *DataService) ProcessData(data string) (string, error) {
	return "", nil
}`,
			expectedContains: []string{
				`data := c.Param("data")`,
				"result0, err := h.service.ProcessData(data)",
			},
		},
		{
			name: "handler with error handling",
			serviceCode: `package service

type OrderService struct{}

//go:generate gortex-gen handler
func (s *OrderService) CreateOrder(c echo.Context, req *CreateOrderRequest) (*Order, error) {
	return nil, nil
}

type CreateOrderRequest struct {
	ProductID string
	Quantity  int
}

type Order struct {
	ID string
}`,
			expectedImports: []string{
				`"github.com/yshengliao/gortex/pkg/errors"`,
			},
			expectedContains: []string{
				"// Use error registry to map business errors to HTTP errors",
				"httpStatus, errResp := errors.HandleBusinessError(err)",
				"if errResp != nil {",
				"return errResp.Send(c, httpStatus)",
				"// Fallback to generic error",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the service code
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tc.serviceCode, parser.ParseComments)
			require.NoError(t, err)

			// Find the method
			var method *ast.FuncDecl
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
					method = fn
					break
				}
			}
			require.NotNil(t, method)

			// Create handler spec
			spec := HandlerSpec{
				PackageName: file.Name.Name,
				MethodName:  method.Name.Name,
				Method:      method,
			}
			
			// Get receiver info
			if method.Recv != nil && len(method.Recv.List) > 0 {
				recv := method.Recv.List[0]
				switch t := recv.Type.(type) {
				case *ast.StarExpr:
					if ident, ok := t.X.(*ast.Ident); ok {
						spec.StructName = ident.Name
						spec.ReceiverType = "*" + ident.Name
					}
				case *ast.Ident:
					spec.StructName = t.Name
					spec.ReceiverType = t.Name
				}
			}

			// Analyze method
			methodInfo, err := AnalyzeMethod(method)
			require.NoError(t, err)

			// Generate handler
			code, err := GenerateHandler(spec, methodInfo)
			require.NoError(t, err)

			// Check imports
			for _, imp := range tc.expectedImports {
				assert.Contains(t, code, imp, "Expected import %s", imp)
			}

			// Check expected content
			for _, expected := range tc.expectedContains {
				assert.Contains(t, code, expected, "Expected to contain: %s", expected)
			}

			// Check not expected content
			for _, notExpected := range tc.notExpected {
				assert.NotContains(t, code, notExpected, "Should not contain: %s", notExpected)
			}

			// Verify it's valid Go code (no syntax errors)
			_, err = parser.ParseFile(token.NewFileSet(), "generated.go", code, 0)
			assert.NoError(t, err, "Generated code should be valid Go")
		})
	}
}

func TestPrepareTemplateData(t *testing.T) {
	// Create a simple method for testing
	code := `package service
type UserService struct{}
func (s *UserService) GetUser(c echo.Context, id string) (*User, error) {
	return nil, nil
}`
	
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	require.NoError(t, err)
	
	var method *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
			method = fn
			break
		}
	}
	require.NotNil(t, method)
	
	spec := HandlerSpec{
		PackageName:  "service",
		StructName:   "UserService",
		MethodName:   "GetUser",
		ReceiverType: "*UserService",
		Method:       method,
	}
	
	methodInfo, err := AnalyzeMethod(method)
	require.NoError(t, err)
	
	data := prepareTemplateData(spec, methodInfo)
	
	assert.Equal(t, "handlers", data.Package)
	assert.Equal(t, "UserServiceHandler", data.HandlerName)
	assert.Equal(t, "UserService", data.StructName)
	assert.Equal(t, "GetUser", data.MethodName)
	assert.Equal(t, "service", data.ServicePackage)
	assert.False(t, data.NeedsBinder)
	
	// Check imports
	assert.True(t, containsImport(data.Imports, "github.com/labstack/echo/v4"))
	assert.True(t, containsImport(data.Imports, "github.com/yshengliao/gortex/pkg/response"))
	
	// Check route
	assert.Len(t, data.Routes, 1)
	route := data.Routes[0]
	assert.Equal(t, "GET", route.HTTPMethod)
	assert.Equal(t, "/user", route.Pattern)
}

func TestCreateRouteInfo(t *testing.T) {
	testCases := []struct {
		name             string
		methodCode       string
		expectedRoute    RouteInfo
	}{
		{
			name: "simple GET method",
			methodCode: `func (h *Handler) GET(c echo.Context) error {
				return nil
			}`,
			expectedRoute: RouteInfo{
				HTTPMethod:        "GET",
				Pattern:          "/",
				MethodName:        "GET",
				HasRequestStruct:  false,
				ReturnsError:      true,
				HasResults:        true,
				ResultVars:        "err",
				ServiceArgs:       []string{"c.Request().Context()"},
			},
		},
		{
			name: "method with request struct and results",
			methodCode: `func (s *Service) CreateUser(c echo.Context, req *CreateRequest) (*User, error) {
				return nil, nil
			}`,
			expectedRoute: RouteInfo{
				HTTPMethod:        "POST",
				Pattern:          "/user",
				MethodName:        "CreateUser",
				HasRequestStruct:  true,
				RequestType:       "*CreateRequest",
				ReturnsError:      true,
				HasResults:        true,
				HasNonErrorResult: true,
				ResultVars:        "result0, err",
				SuccessData:       "result0",
				ServiceArgs:       []string{"c.Request().Context()", "&req"},
			},
		},
		{
			name: "method with path parameters",
			methodCode: `func (s *Service) GetUserProfile(c echo.Context, id string, section string) (*Profile, error) {
				return nil, nil
			}`,
			expectedRoute: RouteInfo{
				HTTPMethod:        "GET",
				Pattern:          "/user-profile",
				MethodName:        "GetUserProfile",
				PathParams: []PathParam{
					{Name: "id", ParamName: "id", Type: "string"},
					{Name: "section", ParamName: "section", Type: "string"},
				},
				ReturnsError:      true,
				HasResults:        true,
				HasNonErrorResult: true,
				ResultVars:        "result0, err",
				SuccessData:       "result0",
				ServiceArgs:       []string{"c.Request().Context()", "id", "section"},
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse method
			code := "package test\ntype T struct{}\n" + tc.methodCode
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", code, 0)
			require.NoError(t, err)
			
			var method *ast.FuncDecl
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					method = fn
					break
				}
			}
			require.NotNil(t, method)
			
			// Create spec
			spec := HandlerSpec{
				MethodName: method.Name.Name,
				Method:     method,
			}
			
			// Analyze method
			methodInfo, err := AnalyzeMethod(method)
			require.NoError(t, err)
			
			// Create route info
			route := createRouteInfo(spec, methodInfo)
			
			// Compare
			assert.Equal(t, tc.expectedRoute.HTTPMethod, route.HTTPMethod)
			assert.Equal(t, tc.expectedRoute.Pattern, route.Pattern)
			assert.Equal(t, tc.expectedRoute.MethodName, route.MethodName)
			assert.Equal(t, tc.expectedRoute.HasRequestStruct, route.HasRequestStruct)
			assert.Equal(t, tc.expectedRoute.RequestType, route.RequestType)
			assert.Equal(t, tc.expectedRoute.ReturnsError, route.ReturnsError)
			assert.Equal(t, tc.expectedRoute.HasResults, route.HasResults)
			assert.Equal(t, tc.expectedRoute.HasNonErrorResult, route.HasNonErrorResult)
			assert.Equal(t, tc.expectedRoute.ResultVars, route.ResultVars)
			assert.Equal(t, tc.expectedRoute.SuccessData, route.SuccessData)
			assert.Equal(t, tc.expectedRoute.ServiceArgs, route.ServiceArgs)
			assert.Equal(t, len(tc.expectedRoute.PathParams), len(route.PathParams))
		})
	}
}

func containsImport(imports []ImportInfo, path string) bool {
	for _, imp := range imports {
		if imp.Path == path {
			return true
		}
	}
	return false
}
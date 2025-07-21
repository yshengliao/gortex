package app

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Debug test to understand what's happening
func TestDebugHandlerAnalysis(t *testing.T) {
	generator := NewRouteCodeGenerator("app")
	
	// Add handlers manually for testing
	generator.Handlers = []HandlerInfo{
		{
			Name:        "API",
			URLPattern:  "/api",
			IsWebSocket: false,
			TypeName:    "TestCodegenHandler",
			Methods:     []MethodInfo{},
		},
	}

	handler := &TestCodegenHandler{}
	handlerType := reflect.TypeOf(handler)
	
	fmt.Printf("Handler type: %+v\n", handlerType)
	fmt.Printf("Handler type name: %s\n", handlerType.Elem().Name())
	fmt.Printf("Looking for type: %s\n", "TestCodegenHandler")
	
	// Test if we can find it
	typeName := handlerType.Elem().Name()
	var handlerInfo *HandlerInfo
	for i := range generator.Handlers {
		fmt.Printf("Comparing %s with %s\n", generator.Handlers[i].TypeName, typeName)
		if generator.Handlers[i].TypeName == typeName {
			handlerInfo = &generator.Handlers[i]
			break
		}
	}
	
	assert.NotNil(t, handlerInfo, "Should find handler")
	
	// Test method discovery
	fmt.Printf("Number of methods: %d\n", handlerType.Elem().NumMethod())
	for i := 0; i < handlerType.Elem().NumMethod(); i++ {
		method := handlerType.Elem().Method(i)
		fmt.Printf("Found method: %s, exported: %t\n", method.Name, method.IsExported())
	}

	// Test specific method lookup
	if method, ok := handlerType.Elem().MethodByName("GET"); ok {
		fmt.Printf("Found GET method: %+v\n", method)
	} else {
		fmt.Printf("GET method not found\n")
	}

	// Debug the full AnalyzeHandlerMethods function
	err := generator.AnalyzeHandlerMethods(handler)
	fmt.Printf("AnalyzeHandlerMethods error: %v\n", err)
	fmt.Printf("Handler methods after analysis: %+v\n", generator.Handlers[0].Methods)
}
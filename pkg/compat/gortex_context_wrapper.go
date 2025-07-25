package compat

import (
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/router"
)

// gortexContextWrapper implements router.Context interface wrapping echo.Context
type gortexContextWrapper struct {
	echoContext echo.Context
}

// Verify interface implementation
var _ router.Context = (*gortexContextWrapper)(nil)

// Implement router.Context methods using echo.Context
func (g *gortexContextWrapper) Param(name string) string {
	return g.echoContext.Param(name)
}

func (g *gortexContextWrapper) QueryParam(name string) string {
	return g.echoContext.QueryParam(name)
}

func (g *gortexContextWrapper) Bind(i interface{}) error {
	return g.echoContext.Bind(i)
}

func (g *gortexContextWrapper) JSON(code int, i interface{}) error {
	return g.echoContext.JSON(code, i)
}

func (g *gortexContextWrapper) String(code int, s string) error {
	return g.echoContext.String(code, s)
}

func (g *gortexContextWrapper) Get(key string) interface{} {
	return g.echoContext.Get(key)
}

func (g *gortexContextWrapper) Set(key string, val interface{}) {
	g.echoContext.Set(key, val)
}

func (g *gortexContextWrapper) Request() interface{} {
	return g.echoContext.Request()
}

func (g *gortexContextWrapper) Response() interface{} {
	return g.echoContext.Response()
}
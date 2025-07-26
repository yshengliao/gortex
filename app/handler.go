package app

import "github.com/yshengliao/gortex/http/context"

// HandlerFunc defines a function to serve HTTP requests.
type HandlerFunc func(c context.Context) error

module github.com/yshengliao/gortex/examples/advanced-tracing

go 1.22

require (
    github.com/yshengliao/gortex v0.4.0-alpha
    github.com/go-redis/redis/v8 v8.11.5
    github.com/lib/pq v1.10.9
    go.uber.org/zap v1.27.0
)

replace github.com/yshengliao/gortex => ../../
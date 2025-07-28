module github.com/yshengliao/gortex/examples/metrics-dashboard

go 1.22

require (
    github.com/prometheus/client_golang v1.19.0
    github.com/yshengliao/gortex v0.4.0-alpha
    go.uber.org/zap v1.27.0
)

replace github.com/yshengliao/gortex => ../../
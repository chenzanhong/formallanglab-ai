package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// ============ 统一的业务操作指标 ============
// module: grammar、Automaton、regex、email、ai、learn、user
var (
	// operation_result_total 记录业务操作的总次数，按模块、操作类型和结果分类
	// 例如: operation_result_total{module="grammar", operation="validate", result="success"}
	operationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "operation_result_total",
			Help: "Total number of business operations by module, operation type and result.",
		},
		[]string{"module", "operation", "result"},
	)

	// operation_duration_seconds 记录业务操作的执行时间分布，按模块和操作类型分类
	// 例如: operation_duration_seconds{module="automaton", operation="minimize"}
	operationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "operation_duration_seconds",
			Help:    "Duration of business operations in seconds.",
			Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"module", "operation"},
	)
)

// ============ HTTP请求指标 ============
var (
	// httpRequestsTotal 记录HTTP请求的总次数，按方法、路径和状态码分类
	// 例如: http_requests_total{method="GET", path="/api/grammar/validate", status="200"}
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDuration 记录HTTP请求的响应时间分布，按方法和路径分类
	// 例如: http_request_duration_seconds{method="POST", path="/api/automaton/minimize"}
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"method", "path"},
	)
)

// PrometheusRegister 将所有定义的指标注册到Prometheus默认注册表中
func PrometheusRegister() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, operationTotal, operationDuration)
}

// IncOperation 增加业务操作计数器
// 参数:
//   - module: 模块名称 (如 "grammar", "automaton")
//   - operation: 操作类型 (如 "validate", "minimize")
//   - result: 操作结果 (如 "success", "failure")
func IncOperation(module, operation, result string) {
	operationTotal.WithLabelValues(module, operation, result).Inc()
}

// ObserveOperationDuration 记录业务操作的执行时间
// 参数:
//   - module: 模块名称 (如 "grammar", "automaton")
//   - operation: 操作类型 (如 "validate", "minimize")
//   - seconds: 操作执行时间(秒)
func ObserveOperationDuration(module, operation string, seconds float64) {
	operationDuration.WithLabelValues(module, operation).Observe(seconds)
}

// HTTPMiddleware HTTP中间件，用于收集HTTP请求指标
// 收集指标包括:
//   - 请求总数 (按方法、路径、状态码)
//   - 请求响应时间分布 (按方法、路径)
func HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// 处理请求
		c.Next()

		// 获取响应状态码
		status := strconv.Itoa(c.Writer.Status())

		// 更新HTTP请求指标
		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}

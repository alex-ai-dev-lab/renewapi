package controller

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/requestguard"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsRegisterOnce sync.Once
	metricsHandler      = promhttp.Handler()
)

func initMetricsCollectors() {
	metricsRegisterOnce.Do(func() {
		prometheus.MustRegister(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "newapi_active_connections",
				Help: "Number of active relay HTTP connections.",
			},
			func() float64 {
				return float64(middleware.GetActiveConnections())
			},
		))
		prometheus.MustRegister(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name:        "newapi_build_info",
				Help:        "Build information for the New API process.",
				ConstLabels: prometheus.Labels{"version": common.Version},
			},
			func() float64 { return 1 },
		))
		for _, collector := range service.StreamRecoveryCollectors() {
			prometheus.MustRegister(collector)
		}
		for _, collector := range requestguard.Collectors() {
			prometheus.MustRegister(collector)
		}
	})
}

func Metrics(c *gin.Context) {
	if !isMetricsRequestAllowed(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	initMetricsCollectors()
	metricsHandler.ServeHTTP(c.Writer, c.Request)
}

func isMetricsRequestAllowed(c *gin.Context) bool {
	token := strings.TrimSpace(os.Getenv("METRICS_TOKEN"))
	if token != "" {
		if bearer := strings.TrimSpace(c.GetHeader("Authorization")); strings.HasPrefix(strings.ToLower(bearer), "bearer ") {
			if strings.TrimSpace(bearer[len("Bearer "):]) == token {
				return true
			}
		}
		if c.GetHeader("X-Metrics-Token") == token {
			return true
		}
	}
	return isPrivateMetricsClientIP(c.ClientIP())
}

func isPrivateMetricsClientIP(clientIP string) bool {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() {
		return true
	}
	return false
}

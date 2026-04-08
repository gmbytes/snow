package node

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const otelTracerName = "github.com/gmbytes/snow/routines/node"

// NodeTracer 返回节点包的 OTel Tracer。
// 调用方需提前通过 otel.SetTracerProvider() 配置全局 TracerProvider。
func NodeTracer() trace.Tracer {
	return otel.Tracer(otelTracerName)
}

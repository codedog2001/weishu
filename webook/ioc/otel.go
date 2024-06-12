package ioc

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"time"
)

// InitOTEL 初始化 OpenTelemetry 的函数。
// 该函数创建并配置 OpenTelemetry 的资源、传播器、跟踪提供者，并返回一个关闭函数，用于清理资源。
// 返回值是一个函数，它接受一个 context.Context 类型的参数，在调用时用于关闭初始化的 OpenTelemetry 跟踪提供者。
func InitOTEL() func(ctx context.Context) {
	// 初始化服务资源，包含服务名称和版本。
	res, err := newResource("webook", "v0.0.1")
	if err != nil {
		panic(err)
	}
	// 初始化传播器，并设置为全局传播器。
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)
	// 创建并配置跟踪提供者，包含资源信息和导出器。
	tp, err := newTranceProvider(res)
	if err != nil {
		panic(err)
	}
	otel.SetTracerProvider(tp)
	// 返回一个关闭函数，用于优雅地关闭跟踪提供者。
	return func(ctx context.Context) {
		_ = tp.Shutdown(ctx)
	}
}

// newResource 创建一个服务资源对象，合并了默认信息，服务名和版本信息传入的时候进行初始化。
func newResource(serviceName, serviceVersion string) (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion)))
}

// newTranceProvider 创建一个新的跟踪提供者，配置了指定的服务资源和导出器。
func newTranceProvider(res *resource.Resource) (*trace.TracerProvider, error) {
	exporter, err := zipkin.New(
		"http://localhost:9411/api/v2/spans")
	if err != nil {
		return nil, err
	}
	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(exporter, trace.WithBatchTimeout(time.Second)),
		trace.WithResource(res))

	return traceProvider, nil
}

// newPropagator 创建一个复合传播器，支持 TraceContext 和 Baggage 标准。
func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{}) //propagation.Baggage是一个用于在分布式系统中传播上下文信息的结构体类型。
}

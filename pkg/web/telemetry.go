package web

import (
	"context"
	"fmt"

	"github.com/fsandov/go-sdk/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func (app *GinApp) setupTelemetry() error {
	cfg := config.Get()

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			attribute.String("service.name", cfg.AppName),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	if app.ginConfig.EnableTracing && app.ginConfig.OTELEndpoint != "" {
		tp, err := newTracerProvider(res, app.ginConfig.OTELEndpoint)
		if err != nil {
			return fmt.Errorf("failed to create tracer provider: %w", err)
		}
		app.tracer = tp

		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		)
	}

	if app.ginConfig.EnableMetrics {
		mp, err := newMeterProvider(res)
		if err != nil {
			return fmt.Errorf("failed to create meter provider: %w", err)
		}
		app.meter = mp

		otel.SetMeterProvider(mp)
	}

	return nil
}

func newTracerProvider(res *resource.Resource, endpoint string) (*sdktrace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exp)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	return tp, nil
}

func newMeterProvider(res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exp, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exp),
	), nil
}

func (app *GinApp) ShutdownTelemetry(ctx context.Context) error {
	var errs []error

	if app.tracer != nil {
		if err := app.tracer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
	}

	if app.meter != nil {
		if err := app.meter.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during telemetry shutdown: %v", errs)
	}

	return nil
}

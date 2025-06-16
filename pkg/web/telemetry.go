package web

import (
	"context"
	"fmt"
	"time"

	"github.com/fsandov/go-sdk/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

type TelemetryConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	TempoEndpoint  string
	Insecure       bool
	EnableTracing  bool
	EnableMetrics  bool
}

func (app *GinApp) setupTelemetry() error {
	cfg := config.Get()

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.AppName),
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

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tp.Shutdown(ctx); err != nil {
				app.logger.Error(context.Background(), "failed to shutdown tracer provider", zap.Error(err))
			}
		}()
	}

	if app.ginConfig.EnableMetrics {
		mp, err := newMeterProvider(res)
		if err != nil {
			return fmt.Errorf("failed to create meter provider: %w", err)
		}
		app.meter = mp

		otel.SetMeterProvider(mp)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := mp.Shutdown(ctx); err != nil {
				app.logger.Error(context.Background(), "failed to shutdown meter provider", zap.Error(err))
			}
		}()
	}

	return nil
}

func newTracerProvider(res *resource.Resource, endpoint string) (*sdktrace.TracerProvider, error) {
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)

	exp, err := otlptrace.New(context.Background(), client)
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

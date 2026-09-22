package ingest

import (
	"fmt"
	"strconv"
	"time"

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
)

type metricRows struct {
	table string
	rows  []map[string]any
}

func mapMetrics(request *collectormetrics.ExportMetricsServiceRequest) []metricRows {
	result := map[string][]map[string]any{}
	for _, resourceMetrics := range request.GetResourceMetrics() {
		resourceAttrs := attributesFromResource(resourceMetrics.GetResource())
		resourceSchema := resourceMetrics.GetSchemaUrl()
		serviceName := resourceAttrs["service.name"]
		for _, scopeMetrics := range resourceMetrics.GetScopeMetrics() {
			scope := scopeMetrics.GetScope()
			scopeAttrs := attributes(scope.GetAttributes())
			for _, metric := range scopeMetrics.GetMetrics() {
				base := map[string]any{
					"ResourceAttributes":    resourceAttrs,
					"ResourceSchemaUrl":     resourceSchema,
					"ScopeName":             scope.GetName(),
					"ScopeVersion":          scope.GetVersion(),
					"ScopeAttributes":       scopeAttrs,
					"ScopeDroppedAttrCount": scope.GetDroppedAttributesCount(),
					"ScopeSchemaUrl":        scopeMetrics.GetSchemaUrl(),
					"ServiceName":           serviceName,
					"MetricName":            metric.GetName(),
					"MetricDescription":     metric.GetDescription(),
					"MetricUnit":            metric.GetUnit(),
				}
				switch data := metric.GetData().(type) {
				case *metrics.Metric_Gauge:
					for _, point := range data.Gauge.GetDataPoints() {
						row := clone(base)
						addNumberPoint(row, point)
						result["otel_metrics_gauge"] = append(result["otel_metrics_gauge"], row)
					}
				case *metrics.Metric_Sum:
					for _, point := range data.Sum.GetDataPoints() {
						row := clone(base)
						addNumberPoint(row, point)
						row["AggregationTemporality"] = int32(data.Sum.GetAggregationTemporality())
						row["IsMonotonic"] = data.Sum.GetIsMonotonic()
						result["otel_metrics_sum"] = append(result["otel_metrics_sum"], row)
					}
				case *metrics.Metric_Histogram:
					for _, point := range data.Histogram.GetDataPoints() {
						row := clone(base)
						row["Attributes"] = attributes(point.GetAttributes())
						row["StartTimeUnix"] = timestamp(point.GetStartTimeUnixNano())
						row["TimeUnix"] = timestamp(point.GetTimeUnixNano())
						row["Count"] = point.GetCount()
						row["Sum"] = point.GetSum()
						row["BucketCounts"] = point.GetBucketCounts()
						row["ExplicitBounds"] = point.GetExplicitBounds()
						row["Flags"] = point.GetFlags()
						row["Min"] = point.GetMin()
						row["Max"] = point.GetMax()
						row["AggregationTemporality"] = int32(data.Histogram.GetAggregationTemporality())
						result["otel_metrics_histogram"] = append(result["otel_metrics_histogram"], row)
					}
				case *metrics.Metric_Summary:
					for _, point := range data.Summary.GetDataPoints() {
						row := clone(base)
						row["Attributes"] = attributes(point.GetAttributes())
						row["StartTimeUnix"] = timestamp(point.GetStartTimeUnixNano())
						row["TimeUnix"] = timestamp(point.GetTimeUnixNano())
						row["Count"] = point.GetCount()
						row["Sum"] = point.GetSum()
						quantiles, values := []float64{}, []float64{}
						for _, value := range point.GetQuantileValues() {
							quantiles = append(quantiles, value.GetQuantile())
							values = append(values, value.GetValue())
						}
						row["ValueAtQuantiles.Quantile"] = quantiles
						row["ValueAtQuantiles.Value"] = values
						row["Flags"] = point.GetFlags()
						result["otel_metrics_summary"] = append(result["otel_metrics_summary"], row)
					}
				}
			}
		}
	}
	out := make([]metricRows, 0, len(result))
	for table, rows := range result {
		out = append(out, metricRows{table: table, rows: rows})
	}
	return out
}

func addNumberPoint(row map[string]any, point *metrics.NumberDataPoint) {
	row["Attributes"] = attributes(point.GetAttributes())
	row["StartTimeUnix"] = timestamp(point.GetStartTimeUnixNano())
	row["TimeUnix"] = timestamp(point.GetTimeUnixNano())
	row["Flags"] = point.GetFlags()
	if point.GetAsInt() != 0 {
		row["Value"] = float64(point.GetAsInt())
	} else {
		row["Value"] = point.GetAsDouble()
	}
}

func clone(input map[string]any) map[string]any {
	output := make(map[string]any, len(input)+8)
	for key, value := range input {
		output[key] = value
	}
	return output
}

func timestamp(nano uint64) string {
	if nano == 0 {
		return "1970-01-01 00:00:00"
	}
	return time.Unix(0, int64(nano)).UTC().Format("2006-01-02 15:04:05")
}

func attributes(values []*common.KeyValue) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		result[value.GetKey()] = anyValue(value.GetValue())
	}
	return result
}

func attributesFromResource(value *resource.Resource) map[string]string {
	if value == nil {
		return map[string]string{}
	}
	return attributes(value.GetAttributes())
}

func anyValue(value *common.AnyValue) string {
	if value == nil {
		return ""
	}
	switch typed := value.GetValue().(type) {
	case *common.AnyValue_StringValue:
		return typed.StringValue
	case *common.AnyValue_BoolValue:
		return strconv.FormatBool(typed.BoolValue)
	case *common.AnyValue_IntValue:
		return strconv.FormatInt(typed.IntValue, 10)
	case *common.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", typed.DoubleValue)
	default:
		return fmt.Sprintf("%v", value.GetValue())
	}
}

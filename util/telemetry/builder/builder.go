package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"sigs.k8s.io/yaml"
)

const generatedBanner string = "// Code generated by util/telemetry/builder. DO NOT EDIT."

//go:embed values.yaml
var valuesYaml []byte

type attribute struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	// Description is a markdown explanation for the documentation. One line only.
	Description string `json:"description"`
}

type allowedAttribute struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional,omitempty"`
}

type metric struct {
	// Name: Metric name, in CamelCaps
	// Will be snake cased for display purposes
	Name string `json:"name"`
	// Description: short description, emitted on the metrics endpoint and added to the documentation. Do not use marrkdown here.
	Description string `json:"description"`
	// ExtendedDescription: Markdown capable further description added to the documentation before attributes
	ExtendedDescription string `json:"extendedDescription,omitempty"`
	// Notes: Markdown capable further description added to the documentation after attributes
	Notes      string             `json:"notes,omitempty"`
	Attributes []allowedAttribute `json:"attributes,omitempty"`
	// Unit: OpenTelemetry unit of measurement https://opentelemetry.io/docs/specs/otel/metrics/api/#instrument-unit
	Unit           string    `json:"unit"`
	Type           string    `json:"type"`
	DefaultBuckets []float64 `json:"defaultBuckets,omitempty"`
}

type attributesList []attribute
type metricsList []metric

type values struct {
	Attributes attributesList `json:"attributes"`
	Metrics    metricsList    `json:"metrics"`
}

func load() values {
	var vals values
	err := yaml.UnmarshalStrict(valuesYaml, &vals)
	if err != nil {
		panic(err)
	}
	return vals
}

var collectedErrors []error

func recordErrorString(err string) {
	collectedErrors = append(collectedErrors, errors.New(err))
}
func recordError(err error) {
	collectedErrors = append(collectedErrors, err)
}

func main() {
	metricsDocs := flag.String("metricsDocs", "", "Path to metrics.md in the docs")
	attributesGo := flag.String("attributesGo", "", "Path to attributes.go in util/telemetry")
	metricsListGo := flag.String("metricsListGo", "", "Path to metrics_list.go in util/telemetry")
	flag.Parse()
	vals := load()
	validate(&vals)
	if len(collectedErrors) == 0 {
		if metricsDocs != nil && *metricsDocs != "" {
			createMetricsDocs(*metricsDocs, &vals.Metrics, &vals.Attributes)
		}
		if attributesGo != nil && *attributesGo != "" {
			createAttributesGo(*attributesGo, &vals.Attributes)
		}
		if metricsListGo != nil && *metricsListGo != "" {
			createMetricsListGo(*metricsListGo, &vals.Metrics)
		}
	}
	if len(collectedErrors) > 0 {
		for _, err := range collectedErrors {
			fmt.Println(err)
		}
		os.Exit(1)
	}
}

func upperToSnake(in string) string {
	runes := []rune(in)
	in = string(append([]rune{unicode.ToLower(runes[0])}, runes[1:]...))
	re := regexp.MustCompile(`[A-Z]`)
	return string(re.ReplaceAllFunc([]byte(in), func(in []byte) []byte {
		return []byte(fmt.Sprintf("_%s", strings.ToLower(string(in[0]))))
	}))
}

func (a *attribute) displayName() string {
	name := a.Name
	if a.DisplayName != "" {
		name = a.DisplayName
	}
	return upperToSnake(name)
}

func validateMetricsAttributes(metrics *metricsList, attributes *attributesList) {
	for _, metric := range *metrics {
		for _, attribute := range metric.Attributes {
			if getAttribByName(attribute.Name, attributes) == nil {
				recordErrorString(fmt.Sprintf("Metric %s: attribute %s not defined", metric.Name, attribute.Name))
			}
		}
	}
}

func validateAttributes(attributes *attributesList) {
	if !slices.IsSortedFunc(*attributes, func(a, b attribute) int {
		return strings.Compare(a.Name, b.Name)
	}) {
		recordErrorString("Attributes must be alphabetically sorted by Name")
	}
	for _, attribute := range *attributes {
		if strings.Contains(attribute.Description, "\n") {
			recordErrorString(fmt.Sprintf("%s: Description must be a single line", attribute.Name))
		}
	}
}

func validateMetrics(metrics *metricsList) {
	if !slices.IsSortedFunc(*metrics, func(a, b metric) int {
		return strings.Compare(a.Name, b.Name)
	}) {
		recordErrorString("Metrics must be alphabetically sorted by Name")
	}
	for _, metric := range *metrics {
		// This is easier than enum+custom JSON unmarshall as this is not critical code
		switch metric.Type {
		case "Float64Histogram":
		case "Float64ObservableGauge":
		case "Int64Counter":
		case "Int64UpDownCounter":
		case "Int64ObservableGauge":
			break
		default:
			recordErrorString(fmt.Sprintf("%s: Invalid metric type %s", metric.Name, metric.Type))
		}
		if strings.Contains(metric.Description, "\n") {
			recordErrorString(fmt.Sprintf("%s: Description must be a single line", metric.Name))
		}
		if strings.HasSuffix(metric.Description, ".") {
			recordErrorString(fmt.Sprintf("%s: Description must not have a trailing period", metric.Name))
		}
	}
}

func validate(vals *values) {
	validateAttributes(&vals.Attributes)
	validateMetrics(&vals.Metrics)
	validateMetricsAttributes(&vals.Metrics, &vals.Attributes)
}

func (m *metric) instrumentType() string {
	return m.Type
}

func (m *metric) displayName() string {
	name := m.Name
	return upperToSnake(name)
}

func getAttribByName(name string, attribs *attributesList) *attribute {
	for _, attrib := range *attribs {
		if name == attrib.Name {
			return &attrib
		}
	}
	return nil
}

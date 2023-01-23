// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package magicexporter

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		"magic",
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelAlpha))
}

type Config struct {
	Name string `mapstructure:"name"`
}

func createDefaultConfig() component.Config {
	return &Config{}
}

type Exporter struct {
	config *Config
}

func createMetricsExporter(context context.Context, set exporter.CreateSettings, baseCfg component.Config) (exporter.Metrics, error) {
	cfg := baseCfg.(*Config)
	return Exporter{
		config: cfg,
	}, nil
}

func (r Exporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (r Exporter) Shutdown(ctx context.Context) error {
	return nil
}

func (r Exporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

type Metric struct {
	Name  string `json:"Name" binding:"required"`
	Value int64  `json:"Value" binding:"required"`
}

var pending map[string][]Metric = make(map[string][]Metric, 0)

func (r Exporter) ConsumeMetrics(ctx context.Context, pm pmetric.Metrics) error {
	fmt.Printf("=== %s: data arrived.\n", r.config.Name)

	resourceMetrics := pm.ResourceMetrics().At(0)
	reqIdAttr, _ := resourceMetrics.Resource().Attributes().Get("RequestId")
	requestId := reqIdAttr.Str()
	lengthAttr, _ := resourceMetrics.Resource().Attributes().Get("Len")
	length := int(lengthAttr.Int())

	metricsSlice := resourceMetrics.ScopeMetrics().At(0).Metrics()
	for i := 0; i < metricsSlice.Len(); i++ {
		m := metricsSlice.At(i)

		jsonMetric := Metric{
			Name:  m.Name(),
			Value: m.Sum().DataPoints().At(0).IntValue(),
		}
		val, ok := pending[requestId]

		if !ok {
			val = make([]Metric, 0)
			pending[requestId] = val
		}

		val = append(val, jsonMetric)
		pending[requestId] = val
	}

	if len(pending[requestId]) == length {
		var body struct {
			RequestId string   `json:"RequestId" binding:"required"`
			Metrics   []Metric `json:"Metrics" binding:"required"`
		}
		body.RequestId = requestId
		body.Metrics = pending[requestId]

		jsonString, _ := json.MarshalIndent(body, "", "\t")

		delete(pending, requestId)

		fmt.Println(string(jsonString))
	} else {
		fmt.Println()
	}
	return nil
}

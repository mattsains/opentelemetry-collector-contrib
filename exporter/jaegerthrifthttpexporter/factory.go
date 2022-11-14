// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jaegerthrifthttpexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter"

import (
	"context"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "jaeger_thrift"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for Jaeger Thrift over HTTP exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesExporter(createTracesExporter, stability))
}

func createDefaultConfig() component.ExporterConfig {
	httpConfig := confighttp.NewDefaultHTTPClientSettings()
	httpConfig.Timeout = exporterhelper.NewDefaultTimeoutSettings().Timeout
	return &Config{
		ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
		HTTPClientSettings: httpConfig,
	}
}

func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config component.ExporterConfig,
) (component.TracesExporter, error) {

	expCfg := config.(*Config)
	_, err := url.ParseRequestURI(expCfg.HTTPClientSettings.Endpoint)
	if err != nil {
		// TODO: Improve error message, see #215
		err = fmt.Errorf("%q config requires a valid \"endpoint\": %w", expCfg.ID().String(), err)
		return nil, err
	}

	if expCfg.HTTPClientSettings.Timeout <= 0 {
		err := fmt.Errorf("%q config requires a positive value for \"timeout\"", expCfg.ID().String())
		return nil, err
	}

	return newTracesExporter(expCfg, set)
}

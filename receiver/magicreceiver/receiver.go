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

package magicreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		"magic",
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelAlpha))
}

type Config struct {
	Name string `mapstructure:"name"`
}

func createDefaultConfig() component.Config {
	return &Config{}
}

type Receiver struct {
	config       *Config
	nextConsumer consumer.Metrics
}

func createMetricsReceiver(
	ctx context.Context,
	set receiver.CreateSettings,
	baseCfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	return Receiver{
		config:       cfg,
		nextConsumer: nextConsumer,
	}, nil
}

func (r Receiver) Start(ctx context.Context, host component.Host) error {
	go func() {
		time.Sleep(time.Second)
		// emit metric
		metrics := pmetric.NewMetrics()

		err := r.nextConsumer.ConsumeMetrics(ctx, metrics)

		if err != nil {
			for _, err := range multierr.Errors(err) {
				json, _ := json.Marshal(err)
				fmt.Printf("==== %s: Got error \n%s\n", r.config.Name, string(json))
			}
		}
	}()
	return nil
}

func (r Receiver) Shutdown(ctx context.Context) error {
	return nil
}

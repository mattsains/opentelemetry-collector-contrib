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
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
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

type nextConsumer func(metrics pmetric.Metrics) error

func (r Receiver) Start(ctx context.Context, host component.Host) error {
	go func() {
		router := setupRouter(func(m pmetric.Metrics) error {
			return r.nextConsumer.ConsumeMetrics(ctx, m)
		})

		router.Run(":8080")
	}()
	return nil
}

func (r Receiver) Shutdown(ctx context.Context) error {
	return nil
}

func setupRouter(nextConsumer nextConsumer) *gin.Engine {
	r := gin.Default()

	// Ping test
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.PUT("metric", func(c *gin.Context) {
		// Parse JSON

		type Metric struct {
			Name  string `json:"Name" binding:"required"`
			Value int64  `json:"Value" binding:"required"`
		}
		var json struct {
			Metrics   []Metric `json:"Metrics" binding:"required"`
			RequestId string   `json:"RequestId" binding:"required"`
		}

		if c.Bind(&json) == nil {
			// json variable now has data
			// emit metric here
			metrics := pmetric.NewMetrics()
			pm := metrics.ResourceMetrics().AppendEmpty()
			sm := pm.ScopeMetrics().AppendEmpty()

			for _, m := range json.Metrics {
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(m.Name)

				s := metric.SetEmptySum()
				s.DataPoints().AppendEmpty().SetIntValue(m.Value)
			}

			pm.Resource().Attributes().PutInt("Len", int64(len(json.Metrics)))
			pm.Resource().Attributes().PutStr("RequestId", json.RequestId)

			err := nextConsumer(metrics)

			//send response
			if err != nil {
				fmt.Printf("==== receiver: Got error \n%s\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"status": "fail"})
			} else {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			}
		}
	})

	return r
}

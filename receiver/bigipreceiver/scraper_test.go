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

package bigipreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"
)

func TestScraperStart(t *testing.T) {
	invalidHTTPConfig := confighttp.NewDefaultHTTPClientSettings()
	invalidHTTPConfig.Endpoint = defaultEndpoint
	invalidHTTPConfig.TLSSetting = configtls.TLSClientSetting{
		TLSSetting: configtls.TLSSetting{
			CAFile: "/non/existent",
		},
	}

	validHTTPConfig := confighttp.NewDefaultHTTPClientSettings()
	validHTTPConfig.Endpoint = defaultEndpoint
	validHTTPConfig.TLSSetting = configtls.TLSClientSetting{}

	testcases := []struct {
		desc        string
		scraper     *bigipScraper
		expectError bool
	}{
		{
			desc: "Bad Config",
			scraper: &bigipScraper{
				cfg: &Config{
					HTTPClientSettings: invalidHTTPConfig,
				},
				settings: componenttest.NewNopTelemetrySettings(),
			},
			expectError: true,
		},
		{
			desc: "Valid Config",
			scraper: &bigipScraper{
				cfg: &Config{
					HTTPClientSettings: validHTTPConfig,
				},
				settings: componenttest.NewNopTelemetrySettings(),
			},
			expectError: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.scraper.start(context.Background(), componenttest.NewNopHost())
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScaperScrape(t *testing.T) {
	testCases := []struct {
		desc              string
		setupMockClient   func(t *testing.T) client
		expectedMetricGen func(t *testing.T) pmetric.Metrics
		expectedErr       error
	}{
		{
			desc: "Nil client",
			setupMockClient: func(t *testing.T) client {
				return nil
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errClientNotInit,
		},
		{
			desc: "Login API Call Failure",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetNewToken", mock.Anything).Return(errors.New("some api error"))
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errors.New("some api error"),
		},
		{
			desc: "Get API Calls All Failure",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetNewToken", mock.Anything).Return(nil)
				mockClient.On("GetVirtualServers", mock.Anything).Return(nil, errors.New("some virtual api error"))
				mockClient.On("GetPools", mock.Anything).Return(nil, errors.New("some pool api error"))
				mockClient.On("GetPoolMembers", mock.Anything, mock.Anything).Return(nil, errCollectedNoPoolMembers)
				mockClient.On("GetNodes", mock.Anything).Return(nil, errors.New("some node api error"))
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				return pmetric.NewMetrics()
			},
			expectedErr: errors.New("failed to scrape any metrics"),
		},
		{
			desc: "Successful Full Empty Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetNewToken", mock.Anything).Return(nil)
				mockClient.On("GetVirtualServers", mock.Anything).Return(&models.VirtualServers{}, nil)
				mockClient.On("GetPools", mock.Anything).Return(&models.Pools{}, nil)
				mockClient.On("GetPoolMembers", mock.Anything, mock.Anything).Return(&models.PoolMembers{}, nil)
				mockClient.On("GetNodes", mock.Anything).Return(&models.Nodes{}, nil)
				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_empty_golden.json")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
		},
		{
			desc: "Successful Partial Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetNewToken", mock.Anything).Return(nil)

				// use helper function from client tests
				data := loadAPIResponseData(t, virtualServersCombinedFile)
				var virtualServers *models.VirtualServers
				err := json.Unmarshal(data, &virtualServers)
				require.NoError(t, err)
				mockClient.On("GetVirtualServers", mock.Anything).Return(virtualServers, nil)

				mockClient.On("GetPools", mock.Anything).Return(nil, errors.New("some pool api error"))
				mockClient.On("GetPoolMembers", mock.Anything, mock.Anything).Return(nil, errCollectedNoPoolMembers)
				mockClient.On("GetNodes", mock.Anything).Return(nil, errors.New("some node api error"))

				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_partial_golden.json")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: scrapererror.NewPartialScrapeError(errors.New("some pool api error; all pool member requests have failed; some node api error"), 0),
		},
		{
			desc: "Successful Partial Collection With Partial Members",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetNewToken", mock.Anything).Return(nil)

				// use helper function from client tests
				data := loadAPIResponseData(t, virtualServersCombinedFile)
				var virtualServers *models.VirtualServers
				err := json.Unmarshal(data, &virtualServers)
				require.NoError(t, err)
				mockClient.On("GetVirtualServers", mock.Anything).Return(virtualServers, nil)

				// use helper function from client tests
				data = loadAPIResponseData(t, poolMembersCombinedFile)
				var poolMembers *models.PoolMembers
				err = json.Unmarshal(data, &poolMembers)
				require.NoError(t, err)
				mockClient.On("GetPoolMembers", mock.Anything, mock.Anything).Return(poolMembers, errors.New("some member api error"))

				mockClient.On("GetPools", mock.Anything).Return(nil, errors.New("some pool api error"))
				mockClient.On("GetNodes", mock.Anything).Return(nil, errors.New("some node api error"))

				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_partial_with_members_golden.json")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: scrapererror.NewPartialScrapeError(errors.New("some pool api error; some member api error; some node api error"), 0),
		},
		{
			desc: "Successful Full Collection",
			setupMockClient: func(t *testing.T) client {
				mockClient := mocks.MockClient{}
				mockClient.On("GetNewToken", mock.Anything).Return(nil)

				// use helper function from client tests
				data := loadAPIResponseData(t, virtualServersCombinedFile)
				var virtualServers *models.VirtualServers
				err := json.Unmarshal(data, &virtualServers)
				require.NoError(t, err)
				mockClient.On("GetVirtualServers", mock.Anything).Return(virtualServers, nil)

				// use helper function from client tests
				data = loadAPIResponseData(t, poolsStatsResponseFile)
				var pools *models.Pools
				err = json.Unmarshal(data, &pools)
				require.NoError(t, err)
				mockClient.On("GetPools", mock.Anything).Return(pools, nil)

				// use helper function from client tests
				data = loadAPIResponseData(t, poolMembersCombinedFile)
				var poolMembers *models.PoolMembers
				err = json.Unmarshal(data, &poolMembers)
				require.NoError(t, err)
				mockClient.On("GetPoolMembers", mock.Anything, mock.Anything).Return(poolMembers, nil)

				// use helper function from client tests
				data = loadAPIResponseData(t, nodesStatsResponseFile)
				var nodes *models.Nodes
				err = json.Unmarshal(data, &nodes)
				require.NoError(t, err)
				mockClient.On("GetNodes", mock.Anything).Return(nodes, nil)

				return &mockClient
			},
			expectedMetricGen: func(t *testing.T) pmetric.Metrics {
				goldenPath := filepath.Join("testdata", "expected_metrics", "metrics_golden.json")
				expectedMetrics, err := golden.ReadMetrics(goldenPath)
				require.NoError(t, err)
				return expectedMetrics
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraper := newScraper(zap.NewNop(), createDefaultConfig().(*Config), componenttest.NewNopReceiverCreateSettings())
			scraper.client = tc.setupMockClient(t)

			actualMetrics, err := scraper.scrape(context.Background())

			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			expectedMetrics := tc.expectedMetricGen(t)

			err = scrapertest.CompareMetrics(expectedMetrics, actualMetrics)
			require.NoError(t, err)
		})
	}
}

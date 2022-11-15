// Copyright The OpenTelemetry Authors
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

package humioexporter

import (
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Helper method to handle boilerplate of loading exporter configuration from file
func loadExporterConfig(t *testing.T, file string, id component.ID) (component.ExporterConfig, *Config) {
	// Initialize exporter factory
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", file))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalExporterConfig(sub, cfg))

	def := factory.CreateDefaultConfig().(*Config)
	require.NotNil(t, def)

	return cfg, def
}

func TestLoadWithDefaults(t *testing.T) {
	// Arrange / Act
	actual, expected := loadExporterConfig(t, "config.yaml", component.NewIDWithName(typeStr, ""))
	expected.Traces.IngestToken = "00000000-0000-0000-0000-0000000000000"
	expected.Endpoint = "https://cloud.humio.com/"

	// Assert
	assert.Equal(t, expected, actual)
}

func TestLoadInvalidCompression(t *testing.T) {
	// Act
	cfg, _ := loadExporterConfig(t, "invalid-compression.yaml", component.NewIDWithName(typeStr, ""))
	err := cfg.Validate()
	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the Content-Encoding header must")
}

func TestLoadInvalidTagStrategy(t *testing.T) {
	// Act
	cfg, _ := loadExporterConfig(t, "invalid-tag.yaml", component.NewIDWithName(typeStr, ""))
	err := cfg.Validate()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tagging strategy must be one of")
}

func TestLoadAllSettings(t *testing.T) {
	expectedHTTPConfig := confighttp.NewDefaultHTTPClientSettings()
	expectedHTTPConfig.Endpoint = "http://localhost:8080/"
	expectedHTTPConfig.Headers = map[string]string{}
	expectedHTTPConfig.Timeout = 10 * time.Second
	expectedHTTPConfig.ReadBufferSize = 4096
	expectedHTTPConfig.WriteBufferSize = 4096
	expectedHTTPConfig.TLSSetting = configtls.TLSClientSetting{
		Insecure:           false,
		InsecureSkipVerify: false,
		ServerName:         "",
		TLSSetting: configtls.TLSSetting{
			CAFile:   "server.crt",
			CertFile: "client.crt",
			KeyFile:  "client.key",
		},
	}
	// Arrange
	expected := &Config{
		ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),

		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      false,
			NumConsumers: 20,
			QueueSize:    2500,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         false,
			InitialInterval: 8 * time.Second,
			MaxInterval:     2 * time.Minute,
			MaxElapsedTime:  5 * time.Minute,
		},

		HTTPClientSettings: expectedHTTPConfig,

		DisableCompression: true,
		Tag:                TagTraceID,
		Logs: LogsConfig{
			IngestToken: "00000000-0000-0000-0000-0000000000000",
			LogParser:   "custom-parser",
		},
		Traces: TracesConfig{
			IngestToken:    "00000000-0000-0000-0000-0000000000001",
			UnixTimestamps: true,
		},
	}

	// Act
	actual, _ := loadExporterConfig(t, "config.yaml", component.NewIDWithName(typeStr, "allsettings"))

	// Assert
	assert.Equal(t, expected, actual)
}

func TestValidate(t *testing.T) {
	httpConfigWithLocalhostURL := confighttp.NewDefaultHTTPClientSettings()
	httpConfigWithLocalhostURL.Endpoint = "http://localhost:8080"

	httpConfigWithLocalhostURLAndHeaders := httpConfigWithLocalhostURL
	httpConfigWithLocalhostURLAndHeaders.Headers = map[string]string{
		"user-agent":       "Humio",
		"content-type":     "application/json",
		"content-encoding": "gzip",
	}

	httpConfigWithEURL := confighttp.NewDefaultHTTPClientSettings()
	httpConfigWithEURL.Endpoint = "e"

	httpConfigWithEURLAndPlaintextHeader := httpConfigWithEURL
	httpConfigWithEURLAndPlaintextHeader.Headers = map[string]string{
		"content-type": "text/plain",
	}

	httpConfigWithEURLAndBearerToken := httpConfigWithEURL
	httpConfigWithEURLAndBearerToken.Headers = map[string]string{
		"authorization": "Bearer mytoken",
	}

	httpConfigWithEURLAndInvalidEncoding := httpConfigWithEURL
	httpConfigWithEURLAndInvalidEncoding.Headers = map[string]string{
		"content-encoding": "compress",
	}

	httpConfigWithEURLAndContentEncoding := httpConfigWithEURL
	httpConfigWithEURLAndContentEncoding.Headers = map[string]string{
		"content-encoding": "gzip",
	}

	httpConfigWithInvalidURL := confighttp.NewDefaultHTTPClientSettings()
	httpConfigWithInvalidURL.Endpoint = "\n\t"

	// Arrange
	testCases := []struct {
		desc    string
		cfg     *Config
		wantErr bool
	}{
		{
			desc: "Valid minimal configuration",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithLocalhostURL,
			},
			wantErr: false,
		},
		{
			desc: "Valid custom headers",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithLocalhostURLAndHeaders,
			},
			wantErr: false,
		},
		{
			desc: "Valid compression disabled",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				DisableCompression: true,
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithLocalhostURL,
			},
			wantErr: false,
		},
		{
			desc: "Missing endpoint",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagNone,
				HTTPClientSettings: confighttp.NewDefaultHTTPClientSettings(),
			},
			wantErr: true,
		},
		{
			desc: "Override tag strategy",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagServiceName,
				HTTPClientSettings: httpConfigWithEURL,
			},
			wantErr: false,
		},
		{
			desc: "Unix time",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithEURL,
				Traces: TracesConfig{
					UnixTimestamps: true,
				},
			},
			wantErr: false,
		},
		{
			desc: "Error creating URLs",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithInvalidURL,
			},
			wantErr: true,
		},
		{
			desc: "Invalid Content-Type header",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithEURLAndPlaintextHeader,
			},
			wantErr: true,
		},
		{
			desc: "User-provided Authorization header",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithEURLAndBearerToken,
			},
			wantErr: true,
		},
		{
			desc: "Invalid content encoding",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithEURLAndInvalidEncoding,
			},
			wantErr: true,
		},
		{
			desc: "Content encoding without compression",
			cfg: &Config{
				ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
				DisableCompression: true,
				Tag:                TagNone,
				HTTPClientSettings: httpConfigWithEURLAndContentEncoding,
			},
			wantErr: true,
		},
	}

	// Act / Assert
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if err := tC.cfg.Validate(); (err != nil) != tC.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tC.wantErr)
			}
		})
	}
}

func TestSanitizeValid(t *testing.T) {
	httpConfig := confighttp.NewDefaultHTTPClientSettings()
	httpConfig.Endpoint = "http://localhost:8080"
	// Arrange
	cfg := &Config{
		ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
		HTTPClientSettings: httpConfig,
	}

	// Act
	err := cfg.sanitize()

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg.unstructuredEndpoint)
	assert.Equal(t, "localhost:8080", cfg.unstructuredEndpoint.Host)
	assert.Equal(t, unstructuredPath, cfg.unstructuredEndpoint.Path)

	assert.NotNil(t, cfg.structuredEndpoint)
	assert.Equal(t, "localhost:8080", cfg.structuredEndpoint.Host)
	assert.Equal(t, structuredPath, cfg.structuredEndpoint.Path)

	assert.Equal(t, map[string]string{
		"content-type":     "application/json",
		"content-encoding": "gzip",
		"user-agent":       "opentelemetry-collector-contrib Humio",
	}, cfg.Headers)
}

func TestSanitizeCustomHeaders(t *testing.T) {
	httpConfig := confighttp.NewDefaultHTTPClientSettings()
	httpConfig.Endpoint = "http://localhost:8080"
	httpConfig.Headers = map[string]string{
		"user-agent":       "Humio",
		"content-type":     "application/json",
		"content-encoding": "gzip",
	}
	// Arrange
	cfg := &Config{
		ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
		HTTPClientSettings: httpConfig,
	}

	// Act
	err := cfg.sanitize()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"content-type":     "application/json",
		"content-encoding": "gzip",
		"user-agent":       "Humio",
	}, cfg.Headers)
}

func TestSanitizeNoCompression(t *testing.T) {
	httpConfig := confighttp.NewDefaultHTTPClientSettings()
	httpConfig.Endpoint = "http://localhost:8080"
	// Arrange
	cfg := &Config{
		ExporterSettings:   config.NewExporterSettings(component.NewID(typeStr)),
		DisableCompression: true,
		HTTPClientSettings: httpConfig,
	}

	// Act
	err := cfg.sanitize()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"content-type": "application/json",
		"user-agent":   "opentelemetry-collector-contrib Humio",
	}, cfg.Headers)
}

func TestGetEndpoint(t *testing.T) {
	httpConfig := confighttp.NewDefaultHTTPClientSettings()
	httpConfig.Endpoint = "http://localhost:8080"
	// Arrange
	expected := &url.URL{
		Scheme: "http",
		Host:   "localhost:8080",
		Path:   structuredPath,
	}

	cfg := Config{
		HTTPClientSettings: httpConfig,
	}

	// Act
	actual, err := cfg.getEndpoint(structuredPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetEndpointError(t *testing.T) {
	httpConfig := confighttp.NewDefaultHTTPClientSettings()
	httpConfig.Endpoint = "\n\t"
	// Arrange
	cfg := Config{
		HTTPClientSettings: httpConfig,
	}

	// Act
	result, err := cfg.getEndpoint(structuredPath)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

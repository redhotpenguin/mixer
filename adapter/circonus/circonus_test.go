// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package circonus

import (
	"context"
	"strings"
	"testing"
	"time"
	"fmt"

	cgm "github.com/circonus-labs/circonus-gometrics"

	"istio.io/mixer/adapter/circonus/config"
	"istio.io/mixer/pkg/adapter/test"
	"istio.io/mixer/template/metric"
)

func TestRecord(t *testing.T) {

	conf := &config.Params{
		SubmissionUrl: "http://fakeurl",
		Metrics: []*config.Params_MetricInfo{
			{Name: "counter", Type: config.COUNTER},
			{Name: "distribution", Type: config.DISTRIBUTION},
			{Name: "gauge", Type: config.GAUGE},
		},
	}

	metrics := map[string]*metric.Type{
		"counter":      {},
		"distribution": {},
		"gauge":        {},
	}


	validGauge := metric.Instance{
		Name: "gauge",
		Value: int64(123),
		Dimensions: make(map[string]interface{}),
	}
	invalidGauge := validGauge
	invalidGauge.Value = "bar"


	validCounter := metric.Instance{
		Name: "counter",
		Value: int64(123),
		Dimensions: make(map[string]interface{}),
	}
	invalidCounter := validCounter
	invalidCounter.Value = 1.0

	requestDuration := &metric.Instance{
		Name: "histogram",
		Value: 146 * time.Millisecond,
	}
	invalidDistribution := &metric.Instance{
		Name: "histogram",
		Value: "not good",
		}
	int64Distribution := &metric.Instance{
		Name: "histogram",
		Value: int64(3459),
	}

	cases := []struct {
		vals      []*metric.Instance
		errString string
	}{
		{[]*metric.Instance{}, ""},
		{[]*metric.Instance{&validGauge}, ""},
		{[]*metric.Instance{&validCounter}, ""},
		{[]*metric.Instance{requestDuration}, ""},
		{[]*metric.Instance{int64Distribution}, ""},
		{[]*metric.Instance{&validCounter, &validGauge}, ""},
		{[]*metric.Instance{&validCounter, &validGauge}, ""},
		{[]*metric.Instance{&invalidCounter}, "could not record"},
		{[]*metric.Instance{&invalidGauge}, "could not record"},
		{[]*metric.Instance{invalidDistribution}, "could not record"},
		{[]*metric.Instance{&validGauge, &invalidGauge}, "could not record"},
	}

	submissionURL := "http://fake.url"
	cmc := &cgm.Config{}
	cmc.CheckManager.Check.SubmissionURL = submissionURL
	cmc.Debug = true
	cm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		t.Errorf("could not create new cgm %v", err)
	}


	for idx, c := range cases {

		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			//info := GetInfo()
			builder := GetInfo().NewBuilder().(*builder)
			builder.SetAdapterConfig(conf)
			builder.SetMetricTypes(metrics)

			metricsHandler, err := builder.Build(context.Background(), test.NewEnv(t))
			if err != nil {
				t.Fatal("failed to build metrics handler: %s", err)
			}

			//cm := &CirconusMetrics{gauges: make(map[string]string), counters: make(map[string]uint64), histograms: make(map[string]*Histogram)}

			handler := metricsHandler.(*handler)
			handler.cm = *cm

			if err := handler.HandleMetric(context.Background(), c.vals); err != nil {
				if c.errString == "" {
					t.Errorf("HandleMetric returned error: %s", err)
				}
				if !strings.Contains(err.Error(), c.errString) {
					t.Errorf("HandleMetric returned error: %s; wanted err containing %s", err.Error(), c.errString)
				}
			}

			if err := handler.Close(); err != nil {
				t.Errorf("handler.Close() returned error: %s", err)
			}

			if c.errString != "" {
				return
			}

//			for _, val := range c.vals {
				//name := val.Name

//			}
			// check cgm metric value here
		})
	}
}

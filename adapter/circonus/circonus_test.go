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

	cgm "github.com/circonus-labs/circonus-gometrics"
	descriptor "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/adapter/circonus/config"
	//	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapter/test"
	"istio.io/mixer/template/metric"
)

func TestNewMetricsAspect(t *testing.T) {
	name := "SubmissionUrl"
	conf := &config.Params{
		SubmissionUrl: "1234",
		Metrics:       map[string]*config.Params_MetricInfo{"a": {NameTemplate: `{{(.apiMethod) "-" (.responseCode)}}`}},
	}

	info := GetInfo()
	b := info.NewBuilder()
	b.SetAdapterConfig(conf)
	env := test.NewEnv(t)
	if _, err := b.Build(context.Background(), env); err != nil {
		t.Errorf("b.NewMetrics(test.NewEnv(t), &config.Params{}) = %s, wanted no err", err)
	}

	logs := env.GetLogs()
	if len(logs) < 1 {
		t.Errorf("len(logs) = %d, wanted at least 1 item logged", len(logs))
	}
	present := false
	for _, l := range logs {
		present = present || strings.Contains(l, name)
	}
	if !present {
		t.Errorf("wanted NewMetricsAspect(env, conf, metrics) to log about '%v', only got logs: %v", name, logs)
	}
}

func TestNewMetricsAspect_InvalidTemplate(t *testing.T) {
	name := "invalidTemplate"
	conf := &config.Params{
		SubmissionUrl: "1234",
		Metrics: map[string]*config.Params_MetricInfo{
			name:      {NameTemplate: `{{ .apiMethod "-" .responseCode }}`}, // fails at execute time, not template parsing time
			"missing": {NameTemplate: "foo"},
		},
	}
	metrics := map[string]*metric.Type{
		name: {Dimensions: map[string]descriptor.ValueType{"apiMethod": descriptor.STRING, "responseCode": descriptor.INT64}},
	}
	info := GetInfo()
	b := info.NewBuilder().(*builder)
	b.SetAdapterConfig(conf)
	b.SetMetricTypes(metrics)
	env := test.NewEnv(t)
	if _, err := b.Build(context.Background(), env); err != nil {
		t.Errorf("NewMetricsAspect(test.NewEnv(t), conf, metrics) = _, %s, wanted no error", err)
	}

	logs := env.GetLogs()
	if len(logs) < 1 {
		t.Errorf("len(logs) = %d, wanted at least 1 item logged", len(logs))
	}
	present := false
	for _, l := range logs {
		present = present || strings.Contains(l, name)
	}
	if !present {
		t.Errorf("wanted NewMetricsAspect(env, conf, metrics) to log template error containing '%s', only got logs: %v", name, logs)
	}
}

func TestNewMetricsAspect_BadTemplate(t *testing.T) {
	conf := &config.Params{
		SubmissionUrl: "1234",
		Metrics:       map[string]*config.Params_MetricInfo{"badtemplate": {NameTemplate: `{{if 1}}`}},
	}
	metrics := map[string]*metric.Type{"badtemplate": {}}
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewMetricsAspect(test.NewEnv(t), config, nil) didn't panic")
		}
	}()

	info := GetInfo()
	b := info.NewBuilder().(*builder)
	b.SetAdapterConfig(conf)
	b.SetMetricTypes(metrics)
	if _, err := b.Build(context.Background(), test.NewEnv(t)); err != nil {
		t.Errorf("NewMetricsAspect(test.NewEnv(t), config, nil) = %v; wanted panic not err", err)
	}
	t.Fail()
}

func TestRecord(t *testing.T) {
	var templateMetricName = "methodCode"
	conf := &config.Params{
		SubmissionUrl: "1234",
	}
	metrics := map[string]*metric.Type{
		templateMetricName: {
			Value:      descriptor.INT64,
			Dimensions: map[string]descriptor.ValueType{"apiMethod": descriptor.STRING, "responseCode": descriptor.INT64},
		},
		"counter":      {},
		"distribution": {},
		"gauge":        {},
	}

	validGauge := metric.Instance{
		Name:       "gauge",
		Value:      int64(123),
		Dimensions: make(map[string]interface{}),
	}
	invalidGauge := validGauge
	invalidGauge.Value = "bar"

	validCounter := metric.Instance{
		Name:       "counter",
		Value:      int64(123),
		Dimensions: make(map[string]interface{}),
	}
	invalidCounter := validCounter
	invalidCounter.Value = 1.0

	requestDuration := &metric.Instance{
		Name:  "distribution",
		Value: 146 * time.Millisecond,
	}
	invalidDistribution := &metric.Instance{
		Name:  "distribution",
		Value: "not good",
	}
	int64Distribution := &metric.Instance{
		Name:  "distribution",
		Value: int64(3459),
	}

	templateMetric := metric.Instance{
		Name:       templateMetricName,
		Value:      int64(1),
		Dimensions: map[string]interface{}{"apiMethod": "methodName", "responseCode": 500},
	}

	//	expectedMetricName := "methodName-500"

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
		{[]*metric.Instance{&validCounter, &validGauge, &templateMetric}, ""},
		{[]*metric.Instance{&invalidCounter}, "could not record"},
		{[]*metric.Instance{&invalidGauge}, "could not record"},
		{[]*metric.Instance{invalidDistribution}, "could not record"},
		{[]*metric.Instance{&validGauge, &invalidGauge}, "could not record"},
		{[]*metric.Instance{&templateMetric, &invalidCounter}, "could not record"},
	}

	for idx, c := range cases {

		info := GetInfo()
		b := info.NewBuilder().(*builder)
		b.SetAdapterConfig(conf)
		b.SetMetricTypes(metrics)

		m, err := b.Build(context.Background(), test.NewEnv(t))
		if err != nil {
			t.Errorf("[%d] newBuilder().NewMetrics(test.NewEnv(t), conf) = _, %s; wanted no err", idx, err)
			continue
		}

		//	cm = &CirconusMetrics{gauges: make(map[string]string), counters: make(map[string]uint64), histograms: make(map[string]*Histogram)}
		cmc := &cgm.Config{}
		cmc.CheckManager.Check.SubmissionURL = "1234"
		cmc.Debug = true
		cm, err := cgm.NewCirconusMetrics(cmc)
		handler := m.(*handler)
		handler.cm = *cm

		if err := handler.HandleMetric(context.Background(), c.vals); err != nil {
			if c.errString == "" {
				t.Errorf("[%d] m.Record(c.vals) = %s; wanted no err", idx, err)
			}
			if !strings.Contains(err.Error(), c.errString) {
				t.Errorf("[%d] m.Record(c.vals) = %s; wanted err containing %s", idx, err.Error(), c.errString)
			}
		}
		if err := m.Close(); err != nil {
			t.Errorf("[%d] m.Close() = %s; wanted no err", idx, err)
		}
		if c.errString != "" {
			continue
		}
		/*
			metrics := rs.GetSent()
			for _, val := range c.vals {
				name := val.Definition.Name
				if val.Definition.Name == templateMetricName {
					name = expectedMetricName
				}
				m := metrics.CollectNamed(name)
				if len(m) < 1 {
					t.Errorf("[%d] metrics.CollectNamed(%s) returned no stats, expected one.\nHave metrics: %v", idx, name, metrics)
				}
			}
		*/
	}
}

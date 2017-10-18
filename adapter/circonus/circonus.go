// Copyright 2017 Istio Authors
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

package circonus

import (
	"context"
	"fmt"
	//	"log"
	//"os"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics"
	multierror "github.com/hashicorp/go-multierror"

	"istio.io/mixer/adapter/circonus/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/metric"
)

type (
	builder struct {
		adpCfg      *config.Params
		metricTypes map[string]*metric.Type
	}

	handler struct {
		cm          cgm.CirconusMetrics
		metricTypes map[string]*metric.Type
		env         adapter.Env
		metrics     map[string]config.Params_MetricInfo_Type
	}
)

// ensure types implement the requisite interfaces
var _ metric.HandlerBuilder = &builder{}
var _ metric.Handler = &handler{}

///////////////// Configuration-time Methods ///////////////

// adapter.HandlerBuilder#Build
func (b *builder) Build(ctx context.Context, env adapter.Env) (adapter.Handler, error) {

	cmc := &cgm.Config{}
	cmc.CheckManager.Check.SubmissionURL = b.adpCfg.SubmissionUrl
	cmc.Debug = true
	//	cmc.Log = log.New(os.Stdout, "", 1)
	cm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		err = env.Logger().Errorf("Could not create NewCirconusMetrics: %v", err)
		return nil, err
	}

	metrics := make(map[string]config.Params_MetricInfo_Type)
	ac := b.adpCfg
	for _, metric := range ac.Metrics {

		metrics[metric.Name] = metric.Type
	}
	return &handler{cm: *cm, metricTypes: b.metricTypes, env: env, metrics: metrics}, nil
}

// adapter.HandlerBuilder#SetAdapterConfig
func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adpCfg = cfg.(*config.Params)
}

// adapter.HandlerBuilder#Validate
func (b *builder) Validate() (ce *adapter.ConfigErrors) {
	return nil
}

// metric.HandlerBuilder#SetMetricTypes
func (b *builder) SetMetricTypes(types map[string]*metric.Type) {
	b.metricTypes = types
}

////////////////// Request-time Methods //////////////////////////
// metric.Handler#HandleMetric
func (h *handler) HandleMetric(ctx context.Context, insts []*metric.Instance) error {
	var result *multierror.Error

	h.env.Logger().Errorf("handling metric")
	for _, inst := range insts {

		if _, ok := h.metricTypes[inst.Name]; !ok {
			h.env.Logger().Errorf("Cannot find Type for instance %s", inst.Name)
			continue
		}
		metricName := inst.Name
		if _, ok := h.metricTypes[metricName]; !ok {
			result = multierror.Append(result, fmt.Errorf("Cannot find Type for instance %s", metricName))
			continue
		}

		metricType, found := h.metrics[metricName]
		if !found {
			result = multierror.Append(result, fmt.Errorf("no type for metric named %s", metricName))
			continue
		}

		switch metricType {

		case config.GAUGE:
			v, ok := inst.Value.(int64)
			if !ok {
				result = multierror.Append(result, fmt.Errorf("could not record gauge '%s': %v", metricName, inst.Value))
				continue
			}
			h.cm.Gauge(metricName, v)

		case config.COUNTER:
			_, ok := inst.Value.(int64)
			if !ok {
				result = multierror.Append(result, fmt.Errorf("could not record counter '%s': %v", metricName, inst.Value))
				continue
			}
			h.cm.Increment(metricName)

		case config.DISTRIBUTION:
			v, ok := inst.Value.(time.Duration)
			if ok {
				h.cm.Timing(metricName, float64(v))
				continue
			}
			vint, ok := inst.Value.(int64)
			if ok {
				h.cm.Timing(metricName, float64(vint))
				continue
			}
			result = multierror.Append(result, fmt.Errorf("could not record distribution '%s': %v", metricName, inst.Value))

		}

	}
	return result.ErrorOrNil()
}

// adapter.Handler#Close
func (h *handler) Close() error {
	return nil
}

////////////////// Bootstrap //////////////////////////
// GetInfo returns the adapter.Info specific to this adapter.
func GetInfo() adapter.Info {
	return adapter.Info{
		Name:        "circonus",
		Description: "Emit metrics to Circonus.com monitoring ingest",
		SupportedTemplates: []string{
			metric.TemplateName,
		},
		NewBuilder:    func() adapter.HandlerBuilder { return &builder{} },
		DefaultConfig: &config.Params{SubmissionUrl: ""},
	}
}

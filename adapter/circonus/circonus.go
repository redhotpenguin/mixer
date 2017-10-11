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
	"io/ioutil"
	"text/template"
	"time"

	cgm "github.com/circonus-labs/circonus-gometrics"
	multierror "github.com/hashicorp/go-multierror"

	"istio.io/mixer/adapter/circonus/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/pool"
	"istio.io/mixer/template/metric"
)

type (
	info struct {
		mtype config.Params_MetricInfo_Type
		tmpl  *template.Template
	}

	builder struct {
		adpCfg      *config.Params
		metricTypes map[string]*metric.Type
	}

	handler struct {
		cm          cgm.CirconusMetrics
		metricTypes map[string]*metric.Type
		env         adapter.Env
		templates   map[string]info // metric name => template
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
	cm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		err = env.Logger().Errorf("Could not create NewCirconusMetrics: %v", err)
		return nil, err
	}

	templates := make(map[string]info)
	ac := b.adpCfg
	for metricName, s := range ac.Metrics {
		def, found := b.metricTypes[metricName]
		if !found {
			env.Logger().Infof("template registered for nonexistent metric '%s'", metricName)
			continue // we don't have a metric that corresponds to this template, skip processing it
		}

		var t *template.Template
		if s.NameTemplate != "" {
			t, _ = template.New(metricName).Parse(s.NameTemplate)
			if err := t.Execute(ioutil.Discard, def.Dimensions); err != nil {
				env.Logger().Warningf(
					"skipping custom metric name for metric '%s', could not satisfy template '%s' with labels '%v': %v",
					metricName, s, def.Dimensions, err)
				continue
			}
		}
		templates[metricName] = info{mtype: s.Type, tmpl: t}
	}

	return &handler{cm: *cm, metricTypes: b.metricTypes, env: env, templates: templates}, nil
}

// adapter.HandlerBuilder#SetAdapterConfig
func (b *builder) SetAdapterConfig(cfg adapter.Config) {
	b.adpCfg = cfg.(*config.Params)
}

// adapter.HandlerBuilder#Validate
func (b *builder) Validate() (ce *adapter.ConfigErrors) {

	ac := b.adpCfg
	for metricName, s := range ac.Metrics {
		if _, err := template.New(metricName).Parse(s.NameTemplate); err != nil {
			ce = ce.Appendf("metricNameTemplateStrings", "failed to parse template '%s' for metric '%s': %v", s, metricName, err)
		}
	}
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

	for _, inst := range insts {

		metricName := inst.Name
		if _, ok := h.metricTypes[metricName]; !ok {
			result = multierror.Append(result, fmt.Errorf("Cannot find Type for instance %s", metricName))
			continue
		}

		t, found := h.templates[metricName]
		if !found {
			result = multierror.Append(result, fmt.Errorf("no info for metric named %s", metricName))
			continue
		}

		if t.tmpl != nil {
			buf := pool.GetBuffer()
			// We don't check the error here because Execute should only fail when the template is invalid; since
			// we check that the templates are parsable in ValidateConfig and further check that they can be executed
			// with the metric's labels in NewMetricsAspect, this should never fail.
			_ = t.tmpl.Execute(buf, inst.Dimensions)
			metricName = buf.String()
			pool.PutBuffer(buf)
		}

		switch t.mtype {

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
func (h *handler) Close() error { return nil }

////////////////// Bootstrap //////////////////////////
// GetInfo returns the adapter.Info specific to this adapter.
func GetInfo() adapter.Info {
	return adapter.Info{
		Name:        "circonus",
		Description: "Emit metrics to Circonus.com monitoring ingest",
		SupportedTemplates: []string{
			metric.TemplateName,
		},
		NewBuilder: func() adapter.HandlerBuilder { return &builder{} },
		DefaultConfig: &config.Params{
			SubmissionUrl: "",
		},
	}
}

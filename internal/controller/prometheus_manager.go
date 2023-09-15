/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"

	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	maxQueryTime = "1y" // Look back 'maxQueryTime' for the most recent value
)

// Functions to get data from the cluster
func (r *LabelGroupReconciler) GetMostRecentValue(susqlPrometheusQuery string) (float64, error) {
	// Return the most recent value found in the table
	client, err := api.NewClient(api.Config{
		Address: r.SusQLPrometheusDatabaseUrl,
	})

	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//queryString := fmt.Sprintf("max_over_time(%s[%s])", susqlPrometheusQuery, maxQueryTime)
	queryString := fmt.Sprintf("max_over_time(%s[%s])", susqlPrometheusQuery, maxQueryTime)
	results, warnings, err := v1api.Query(ctx, queryString, time.Now(), v1.WithTimeout(0*time.Second))

	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}

	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		return 0.0, err
	}

	return float64(results.(model.Vector)[0].Value), err
}

func (r *LabelGroupReconciler) GetMetricValuesForPodNames(metricName string, podNames []string) (map[string]float64, error) {
	client, err := api.NewClient(api.Config{
		Address: r.KeplerPrometheusUrl,
	})

	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryString := fmt.Sprintf("%s{pod_name=~\"%s\",mode=\"dynamic\"}", metricName, strings.Join(podNames, "|"))
	results, warnings, err := v1api.Query(ctx, queryString, time.Now(), v1.WithTimeout(0*time.Second))

	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		return nil, err
	}

	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}

	metricValues := make(map[string]float64, len(results.(model.Vector)))

	for _, result := range results.(model.Vector) {
		metricValues[string(result.Metric["container_id"])] = float64(result.Value)
	}

	return metricValues, nil
}

type SusqlMetrics struct {
	totalEnergy *prometheus.GaugeVec
}

var (
	susqlMetrics = &SusqlMetrics{
		totalEnergy: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "susql",
			Name:      "total_energy_joules",
			Help:      "Accumulated energy over time for set of labels",
		}, susqlPrometheusLabelNames),
	}

	prometheusRegistry *prometheus.Registry
	prometheusHandler http.Handler
)

func (r *LabelGroupReconciler) SetAggregatedEnergyForLabels(totalEnergy float64, prometheusLabels map[string]string) error {
	// Initiate the exporting of prometheus metrics for the energy
	if prometheusRegistry == nil {
		prometheusRegistry = prometheus.NewRegistry()
		prometheusRegistry.MustRegister(susqlMetrics.totalEnergy)

		prometheusHandler = promhttp.HandlerFor(prometheusRegistry, promhttp.HandlerOpts{Registry: prometheusRegistry})
		http.Handle("/metrics", prometheusHandler)

		if metricsUrl, parseErr := url.Parse(r.SusQLPrometheusMetricsUrl); parseErr == nil {
			fmt.Printf("Serving metrics at '%s:%s'...", metricsUrl.Hostname(), metricsUrl.Port())

			go func () {
				err := http.ListenAndServe(metricsUrl.Hostname() + ":" + metricsUrl.Port(), nil)

				if err != nil {
					panic("ListenAndServe: " + err.Error())
				}
			}()
		}
	}

	susqlMetrics.totalEnergy.With(prometheusLabels).Set(totalEnergy)

	return nil
}
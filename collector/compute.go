package collector

import (
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/services"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	computeUpDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "compute", "up"),
		"Status of compute services",
		[]string{"id", "binary", "service_host", "zone"}, nil,
	)
	computeEnabledDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "compute", "enabled"),
		"Admin status of compute services",
		[]string{"id", "binary", "service_host", "zone"}, nil,
	)
	computeLastSeenDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "compute", "last_seen"),
		"Last time the service was seen by OpenStack",
		[]string{"id", "binary", "service_host", "zone"}, nil,
	)
)

func init() {
	registerCollector("compute", newComputeCollector)
}

func newComputeCollector(provider *gophercloud.ProviderClient, opts gophercloud.EndpointOpts) (Collector, error) {
	client, err := openstack.NewComputeV2(provider, opts)
	if err != nil {
		return nil, err
	}
	return computeCollector{client}, nil
}

type computeCollector struct {
	client *gophercloud.ServiceClient
}

func (c computeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- computeUpDesc
	ch <- computeEnabledDesc
}

func (c computeCollector) Update(ch chan<- prometheus.Metric) error {
	pager := services.List(c.client)
	if pager.Err != nil {
		return fmt.Errorf("Unable to get data: %v", pager.Err)
	}

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		svcs, err := services.ExtractServices(page)
		if err != nil {
			return false, err
		}

		for _, service := range svcs {
			var state float64
			var enabled float64
			if service.State == "up" {
				state = 1
			}
			// Disabled by admin. So it is neither up (1) nor down (0).
			if service.Status == "enabled" {
				enabled = 1
			}

			ch <- prometheus.MustNewConstMetric(computeUpDesc, prometheus.GaugeValue, state, service.ID, service.Binary, service.Host, service.Zone)
			ch <- prometheus.MustNewConstMetric(computeEnabledDesc, prometheus.GaugeValue, enabled, service.ID, service.Binary, service.Host, service.Zone)
			ch <- prometheus.MustNewConstMetric(computeLastSeenDesc, prometheus.CounterValue, float64(service.UpdatedAt.Unix()), service.ID, service.Binary, service.Host, service.Zone)
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("During fetching services: %v", err)
	}

	return nil
}

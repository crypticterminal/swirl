package biz

import (
	"fmt"
	"strings"
	"time"

	"github.com/cuigh/auxo/data"
	"github.com/cuigh/auxo/log"
	"github.com/cuigh/auxo/net/web"
	"github.com/cuigh/swirl/biz/docker"
	"github.com/cuigh/swirl/dao"
	"github.com/cuigh/swirl/model"
)

// Chart return a chart biz instance.
var Chart = &chartBiz{}

type chartBiz struct {
}

func (b *chartBiz) List() (charts []*model.Chart, err error) {
	do(func(d dao.Interface) {
		charts, err = d.ChartList()
	})
	return
}

func (b *chartBiz) Create(chart *model.Chart, user web.User) (err error) {
	do(func(d dao.Interface) {
		//chart.CreatedAt = time.Now()
		//chart.UpdatedAt = chart.CreatedAt
		err = d.ChartCreate(chart)
	})
	return
}

func (b *chartBiz) Delete(id string, user web.User) (err error) {
	do(func(d dao.Interface) {
		err = d.ChartDelete(id)
	})
	return
}

func (b *chartBiz) Get(name string) (chart *model.Chart, err error) {
	do(func(d dao.Interface) {
		chart, err = d.ChartGet(name)
	})
	return
}

func (b *chartBiz) Update(chart *model.Chart, user web.User) (err error) {
	do(func(d dao.Interface) {
		//chart.UpdatedAt = time.Now()
		err = d.ChartUpdate(chart)
	})
	return
}

func (b *chartBiz) GetServiceCharts(name string) (charts []*model.Chart, err error) {
	service, _, err := docker.ServiceInspect(name)
	if err != nil {
		return nil, err
	}

	var categories []string
	if label := service.Spec.Labels["swirl.metrics"]; label != "" {
		categories = strings.Split(label, ",")
	}

	charts = append(charts, model.NewChart("cpu", "CPU", "${name}", `rate(container_cpu_user_seconds_total{container_label_com_docker_swarm_service_name="%s"}[5m]) * 100`, "percent:100"))
	charts = append(charts, model.NewChart("memory", "Memory", "${name}", `container_memory_usage_bytes{container_label_com_docker_swarm_service_name="%s"}`, "size:bytes"))
	charts = append(charts, model.NewChart("network_in", "Network Receive", "${name}", `sum(irate(container_network_receive_bytes_total{container_label_com_docker_swarm_service_name="%s"}[5m])) by(name)`, "size:bytes"))
	charts = append(charts, model.NewChart("network_out", "Network Send", "${name}", `sum(irate(container_network_transmit_bytes_total{container_label_com_docker_swarm_service_name="%s"}[5m])) by(name)`, "size:bytes"))
	for _, c := range categories {
		if c == "java" {
			charts = append(charts, model.NewChart("threads", "Threads", "${instance}", `jvm_threads_current{service="%s"}`, ""))
			charts = append(charts, model.NewChart("gc_duration", "GC Duration", "${instance}", `rate(jvm_gc_collection_seconds_sum{service="%s"}[1m])`, "time:s"))
		} else if c == "go" {
			charts = append(charts, model.NewChart("threads", "Threads", "${instance}", `go_threads{service="%s"}`, ""))
			charts = append(charts, model.NewChart("goroutines", "Goroutines", "${instance}", `go_goroutines{service="%s"}`, ""))
			charts = append(charts, model.NewChart("gc_duration", "GC Duration", "${instance}", `sum(go_gc_duration_seconds{service="%s"}) by (instance)`, "time:s"))
		}
	}
	for i, c := range charts {
		charts[i].Query = fmt.Sprintf(c.Query, name)
	}
	return
}

// nolint: gocyclo
func (b *chartBiz) Panel(panel model.ChartPanel) (charts []*model.Chart, err error) {
	do(func(d dao.Interface) {
		if len(panel.Charts) == 0 {
			return
		}

		names := make([]string, len(panel.Charts))
		for i, c := range panel.Charts {
			names[i] = c.Name
		}

		var cs []*model.Chart
		cs, err = d.ChartBatch(names...)
		if err != nil {
			return
		}

		if len(cs) > 0 {
			m := make(map[string]*model.Chart)
			for _, c := range cs {
				m[c.Name] = c
			}
			for _, c := range panel.Charts {
				if chart := m[c.Name]; chart != nil {
					if c.Width > 0 {
						chart.Width = c.Width
					}
					if c.Height > 0 {
						chart.Height = c.Height
					}
					if len(c.Colors) > 0 {
						chart.Colors = c.Colors
					}
					charts = append(charts, chart)
				}
			}
		}
	})
	return
}

// todo:
func (b *chartBiz) FetchDatas(charts []*model.Chart, period time.Duration) (data.Map, error) {
	datas := data.Map{}
	end := time.Now()
	start := end.Add(-period)
	for _, chart := range charts {
		switch chart.Type {
		case "line", "bar":
			m, err := Metric.GetMatrix(chart.Query, chart.Label, start, end)
			if err != nil {
				log.Get("metric").Error(err)
			} else {
				datas[chart.Name] = m
			}
		case "pie", "table":
			m, err := Metric.GetVector(chart.Query, chart.Label, end)
			if err != nil {
				log.Get("metric").Error(err)
			} else {
				datas[chart.Name] = m
			}
		case "value":
		}
	}
	return datas, nil
}
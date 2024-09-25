package main

import (
	"fmt"
	"github.com/panjf2000/ants/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"strings"
	"sync"
	"time"
)

type MetricsReporter struct {
	wg            *sync.WaitGroup
	pool          *ants.PoolWithFunc
	elasticPoster *ElasticPoster
}

type ReportingTask struct {
	containerMetrics v1beta1.ContainerMetrics
	forPod           v1beta1.PodMetrics
	podData          *v1.Pod
}

func NewMetricsReporter(threadsNum int) *MetricsReporter {
	wg := &sync.WaitGroup{}

	reporter := &MetricsReporter{
		wg:            wg,
		elasticPoster: NewElasticPoster(),
	}

	pool, _ := ants.NewPoolWithFunc(threadsNum, func(i interface{}) {
		task := i.(ReportingTask)
		reporter.recordMetric(task.containerMetrics, task.forPod, task.podData)
		wg.Done()
	})

	reporter.pool = pool

	return reporter
}

func (m *MetricsReporter) RecordMetrics(metricsData []v1beta1.PodMetrics, podsData []v1.Pod) {
	var podsMap = make(map[string]v1.Pod)
	for _, podData := range podsData {
		podsMap[podData.Name] = podData
	}

	for _, metricData := range metricsData {
		for _, container := range metricData.Containers {
			m.wg.Add(1)
			_ = m.pool.Invoke(ReportingTask{containerMetrics: container, forPod: metricData, podData: m.getPodData(podsMap, metricData.Name)})
		}
	}
}

func (m *MetricsReporter) getPodData(podsMap map[string]v1.Pod, podName string) *v1.Pod {
	if _, ok := podsMap[podName]; !ok {
		return nil
	}

	pod := podsMap[podName]
	return &pod
}

func (m *MetricsReporter) recordMetric(containerMetrics v1beta1.ContainerMetrics, forPod v1beta1.PodMetrics, podData *v1.Pod) {
	fmt.Printf("Container name: %s of %s, CPU usage: %d, mem: %d\n", containerMetrics.Name, containerMetrics.Name, containerMetrics.Usage.Cpu().MilliValue(), containerMetrics.Usage.Memory().Value())

	podInfo := PodInfo{
		Timestamp:      time.Now().Format(time.RFC3339),
		PodName:        forPod.Name,
		PodAnnotations: forPod.Annotations,
		Metrics: PodMetricsInfo{
			ContainerName: containerMetrics.Name,
			Cpu:           containerMetrics.Usage.Cpu().AsApproximateFloat64(),
			Memory:        containerMetrics.Usage.Memory().Value(),
		},
	}

	if podData != nil {
		podInfo.Spec = CustomPodSpec(podData.Spec)
		podInfo.PodAnnotations = podData.Annotations
		podInfo.PodStatus = podData.Status
		podInfo.PodKind = podData.Kind
		podInfo.PodLabels = flattenLabels(podData.Labels)
		podInfo.PodNamespace = podData.Namespace
	}

	m.elasticPoster.PostPod(podInfo)
}

func flattenLabels(labels map[string]string) map[string]string {
	flatLabels := make(map[string]string)
	for key, value := range labels {
		parsedKey := strings.ReplaceAll(key, ".", "-")
		flatLabels[parsedKey] = value
	}
	return flatLabels
}

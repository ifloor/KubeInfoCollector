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
	podWg         *sync.WaitGroup
	podPool       *ants.PoolWithFunc
	nodeWg        *sync.WaitGroup
	nodePool      *ants.PoolWithFunc
	elasticPoster *ElasticPoster
}

type PodReportingTask struct {
	containerMetrics v1beta1.ContainerMetrics
	forPod           v1beta1.PodMetrics
	podData          *v1.Pod
}

type NodeReportingTask struct {
	nodeMetrics v1beta1.NodeMetrics
}

func NewMetricsReporter(threadsNum int) *MetricsReporter {
	reporter := &MetricsReporter{
		podWg:         &sync.WaitGroup{},
		nodeWg:        &sync.WaitGroup{},
		elasticPoster: NewElasticPoster(),
	}

	podPool, _ := ants.NewPoolWithFunc(threadsNum, func(i interface{}) {
		task := i.(PodReportingTask)
		reporter.recordPodMetric(task.containerMetrics, task.forPod, task.podData)
		reporter.podWg.Done()
	})
	reporter.podPool = podPool

	//
	nodePool, _ := ants.NewPoolWithFunc(threadsNum, func(i interface{}) {
		task := i.(NodeReportingTask)
		reporter.recordNodeMetric(task.nodeMetrics)
		reporter.nodeWg.Done()
	})
	reporter.nodePool = nodePool

	return reporter
}

func (m *MetricsReporter) RecordPodMetrics(metricsData []v1beta1.PodMetrics, podsData []v1.Pod) {
	var podsMap = make(map[string]v1.Pod)
	for _, podData := range podsData {
		podsMap[podData.Name] = podData
	}

	for _, metricData := range metricsData {
		for _, container := range metricData.Containers {
			m.podWg.Add(1)
			_ = m.podPool.Invoke(PodReportingTask{containerMetrics: container, forPod: metricData, podData: m.getPodData(podsMap, metricData.Name)})
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

func (m *MetricsReporter) recordPodMetric(containerMetrics v1beta1.ContainerMetrics, forPod v1beta1.PodMetrics, podData *v1.Pod) {
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

func (m *MetricsReporter) RecordNodeMetrics(nodesData []v1beta1.NodeMetrics) {
	for _, nodeData := range nodesData {
		m.nodeWg.Add(1)
		_ = m.nodePool.Invoke(NodeReportingTask{nodeMetrics: nodeData})
	}
}

func (m *MetricsReporter) recordNodeMetric(nodeMetric v1beta1.NodeMetrics) {
	fmt.Printf("Node name: %s, CPU usage: %d, mem: %d\n", nodeMetric.Name, nodeMetric.Usage.Cpu().MilliValue(), nodeMetric.Usage.Memory().Value())

	nodeInfo := NodeInfo{
		Timestamp: time.Now().Format(time.RFC3339),
		Metadata:  nodeMetric.ObjectMeta,
		Metrics: NodeMetricsInfo{
			Cpu:    nodeMetric.Usage.Cpu().AsApproximateFloat64(),
			Memory: nodeMetric.Usage.Memory().Value(),
		},
	}

	m.elasticPoster.PostNode(nodeInfo)
}

func flattenLabels(labels map[string]string) map[string]string {
	flatLabels := make(map[string]string)
	for key, value := range labels {
		parsedKey := strings.ReplaceAll(key, ".", "-")
		flatLabels[parsedKey] = value
	}
	return flatLabels
}

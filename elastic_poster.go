package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olivere/elastic/v7"
	v1 "k8s.io/api/core/v1"
	"log"
	"os"
)

type ElasticPoster struct {
	client       *elastic.Client
	elasticIndex string
}

type PodInfo struct {
	Timestamp      string            `json:"@timestamp"`
	PodName        string            `json:"pod.name"`
	PodAnnotations map[string]string `json:"pod.annotations"`
	PodStatus      v1.PodStatus      `json:"pod.status"`
	PodKind        string            `json:"pod.kind"`
	PodLabels      map[string]string `json:"pod.labels"`
	PodNamespace   string            `json:"pod.namespace"`
	Metrics        PodMetricsInfo    `json:"metrics"`
	Spec           CustomPodSpec     `json:"spec"`
}

type PodMetricsInfo struct {
	ContainerName string  `json:"containerName"`
	Cpu           float64 `json:"cpu"`
	Memory        int64   `json:"memory"`
}

// CustomPodSpec is a wrapper around v1.PodSpec to omit the startupProbe field
type CustomPodSpec v1.PodSpec

// MarshalJSON omits the startupProbe field from the JSON output
func (c CustomPodSpec) MarshalJSON() ([]byte, error) {
	type Alias CustomPodSpec
	aux := struct {
		Alias
		Containers []struct {
			v1.Container
			StartupProbe interface{} `json:"startupProbe,omitempty"`
		} `json:"containers"`
	}{
		Alias: Alias(c),
	}

	for _, container := range c.Containers {
		aux.Containers = append(aux.Containers, struct {
			v1.Container
			StartupProbe interface{} `json:"startupProbe,omitempty"`
		}{
			Container: container,
		})
	}

	return json.Marshal(aux)
}

func NewElasticPoster() *ElasticPoster {
	elasticUrl := os.Getenv("ELASTIC_URL")
	if elasticUrl == "" {
		panic("ELASTIC_URL environment variable is required")
	}

	elasticUsername := os.Getenv("ELASTICSEARCH_USERNAME")
	if elasticUsername == "" {
		panic("ELASTICSEARCH_USERNAME environment variable is required")
	}

	elasticPassword := os.Getenv("ELASTICSEARCH_PASSWORD")
	if elasticPassword == "" {
		panic("ELASTICSEARCH_PASSWORD environment variable is required")
	}

	elasticIndex := os.Getenv("ELASTICSEARCH_INDEX")
	if elasticIndex == "" {
		elasticIndex = "kube-monitoring"
	}

	client, err := elastic.NewClient(
		elastic.SetURL(elasticUrl),
		elastic.SetSniff(false),
		elastic.SetBasicAuth(elasticUsername, elasticPassword),
	)
	if err != nil {
		log.Fatalf("Error creating Elasticsearch client: %v", err)
	}

	return &ElasticPoster{
		client:       client,
		elasticIndex: elasticIndex,
	}
}

func (e *ElasticPoster) PostPod(data PodInfo) {
	ctx := context.Background()
	_, err := e.client.Index().
		Index("kube-monitoring").
		BodyJson(data).
		Do(ctx)
	if err != nil {
		log.Printf("Error posting pod data to Elasticsearch: %v", err)
	}

	fmt.Printf("Successfully posted pod data: %s\n", data.PodName)
}

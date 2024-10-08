package main

import (
	"context"
	"flag"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	fmt.Println("Starting processing")

	customPath := os.Getenv("KUBECONFIG")
	var kubeconfig *string

	if customPath == "InClusterConfig" {
		v := ""
		kubeconfig = &v
	} else if customPath != "" {
		kubeconfig = flag.String("kubeconfig", customPath, "(optional) absolute path to the kubeconfig file")
	} else if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()

	var config *rest.Config
	var err error

	if *kubeconfig != "" {
		fmt.Printf("Trying to build configs from path %s\n", *kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			fmt.Printf("Error when trying to build configs from path %s\n", *kubeconfig)
			panic(err.Error())
		}
	} else {
		fmt.Printf("Trying to build configs from rest InClusterConfig\n")
		config, err = rest.InClusterConfig()
		if err != nil {
			fmt.Printf("Error when trying to build configs from rest InClusterConfig %s\n", *kubeconfig)
			panic(err.Error())
		}
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Error building clientSet")
		panic(err.Error())
	}
	metricsSet, err := metrics.NewForConfig(config)
	if err != nil {
		fmt.Println("Error building metricsSet")
		panic(err.Error())
	}

	numThreadsStr := os.Getenv("NUMBER_THREADS")
	var numThreads int64 = 10
	if numThreadsStr != "" {
		numThreads, err = strconv.ParseInt(numThreadsStr, 10, 64)
		if err != nil {
			fmt.Printf("Error parsing NUMBER_THREADS: %s\n", err.Error())
			numThreads = 10
		}
	}

	reporter := NewMetricsReporter(int(numThreads))

	cycleSeconds := 60
	cycleSecondsStr := os.Getenv("CYCLE_SECONDS")
	if cycleSecondsStr != "" {
		cycleSeconds, err = strconv.Atoi(cycleSecondsStr)
		if err != nil {
			fmt.Printf("Error parsing CYCLE_SECONDS: %s\n", err.Error())
			panic(err.Error())
		}
	}

	for {
		startTime := time.Now()
		fmt.Println("Running collection cycle")
		pods := getPods(clientSet)

		podMetricsData, nodeMetricsData := getMetrics(metricsSet)
		reporter.RecordPodMetrics(podMetricsData, pods)
		reporter.RecordNodeMetrics(nodeMetricsData)

		elapsedSeconds := time.Since(startTime).Seconds()
		neededSleep := cycleSeconds - int(math.Round(elapsedSeconds))
		if neededSleep > 0 {
			fmt.Printf("Elapsed %f seconds. Sleeping for %d seconds\n", elapsedSeconds, neededSleep)
			time.Sleep(time.Duration(neededSleep) * time.Second)
		} else {
			fmt.Printf("Cycle took longer than %d seconds. No sleep needed\n", cycleSeconds)
		}
	}
}

func getPods(clientSet *kubernetes.Clientset) []v1.Pod {
	pods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	return pods.Items
}

func getMetrics(metricsSet *metrics.Clientset) ([]v1beta1.PodMetrics, []v1beta1.NodeMetrics) {
	podMetricsData, err := metricsSet.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error getting pod metrics: %s", err.Error())
		panic(err.Error())
	}

	nodeMetricsData, err := metricsSet.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error getting node metrics: %s", err.Error())
		panic(err.Error())
	}
	return podMetricsData.Items, nodeMetricsData.Items
}

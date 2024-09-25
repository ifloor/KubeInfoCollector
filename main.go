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
	"os"
	"path/filepath"
	"strconv"
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

	for {
		fmt.Println("Running collection cycle")
		pods := getPods(clientSet)

		metricsData := getMetrics(metricsSet)
		reporter.RecordMetrics(metricsData, pods)

		break
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

func getMetrics(metricsSet *metrics.Clientset) []v1beta1.PodMetrics {
	//mc.MetricsV1beta1().NodeMetricses().Get("your node name", metav1.GetOptions{})
	//mc.MetricsV1beta1().NodeMetricses().List(metav1.ListOptions{})
	//metricsSet.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(metav1.ListOptions{})
	metricsData, err := metricsSet.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	//mc.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).Get("your pod name", metav1.GetOptions{})

	if err != nil {
		fmt.Printf("Error getting metrics: %s", err.Error())
		panic(err.Error())
	}

	return metricsData.Items
}

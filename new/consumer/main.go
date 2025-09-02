package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/yshngg/k8seventlab/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

func main() {
	// Initialize klog flags and ensure logs are flushed on exit
	klog.InitFlags(flag.CommandLine)
	defer klog.Flush()

	// Setup kubeconfig path from home directory or command line flag
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// Initialize Kubernetes client configuration
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Watch for events in the specified namespace
	events, err := clientset.EventsV1().Events(common.EventNamespace).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	// Process events from the watch channel
	for event := range events.ResultChan() {
		e, ok := event.Object.(*corev1.Event)
		if !ok {
			// Handle non-event status messages
			s := event.Object.(*metav1.Status)
			fmt.Println(s.Code, s.Reason, s.Message, s.Details)
			continue
		}
		// Process events with matching reason
		if e.Reason == common.EventReason {
			if e.Series != nil {
				fmt.Println(e.Reason, e.Message, e.EventTime, e.Series.LastObservedTime, e.Series.Count)
				continue
			}
			fmt.Println(e.Reason, e.Message, e.EventTime)
		}
	}
}

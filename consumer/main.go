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
	// Initialize klog flags for logging
	klog.InitFlags(flag.CommandLine)
	defer klog.Flush()

	// Set up kubeconfig path either from home directory or command line flag
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// Build kubernetes configuration from the kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// Create Kubernetes clientset for API operations
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Set up event watcher for CoreV1 events in the specified namespace
	events, err := clientset.CoreV1().Events(common.EventNamespace).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	// Process incoming events
	for event := range events.ResultChan() {
		// Try to cast the event object to CoreV1 Event
		e, ok := event.Object.(*corev1.Event)
		if !ok {
			// Handle non-event status messages
			s := event.Object.(*metav1.Status)
			fmt.Println(s.Code, s.Reason, s.Message, s.Details)
			continue
		}
		// Filter and display events matching our event reason
		if e.Reason == common.EventReason {
			fmt.Println(e.Reason, e.Message, e.FirstTimestamp, e.LastTimestamp, e.Count)
		}
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yshngg/k8seventlab/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
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

	// Set up signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Initialize event broadcaster for CoreV1 events
	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))

	// Configure event recording with scheme and source
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	eventRecorder := eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: common.ComponentEventLab})
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: clientset.CoreV1().Events("")})

	// Create a ConfigMap to be the subject of our events
	configmap, err := clientset.CoreV1().ConfigMaps(common.ConfigMapNamespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: common.ConfigMapNamespace,
			Name:      common.ConfigMapName,
		},
	}, metav1.CreateOptions{})

	// Set up cleanup function to remove events and ConfigMap on exit
	defer func(configmap *corev1.ConfigMap) {
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = clientset.CoreV1().
			Events(configmap.Namespace).
			DeleteCollection(
				ctx,
				metav1.DeleteOptions{},
				metav1.ListOptions{
					FieldSelector: fmt.Sprintf("involvedObject.uid=%s", configmap.UID),
				})
		if err != nil {
			klog.Error(err)
		}
		err = clientset.CoreV1().
			ConfigMaps(configmap.Namespace).
			Delete(ctx, configmap.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err)
			return
		}
		klog.Info("clean up residual resources successfully")
	}(configmap)

	if err != nil {
		klog.Error(err)
		stop()
		return
	}

	// Start event generation loop
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-ticker.C:
			// Generate an event every second with incremental counter
			eventRecorder.Event(configmap, common.EventType, common.EventReason, fmt.Sprintf("%s %d", common.EventMessage, i))
			i++
		case <-ctx.Done():
			// Handle shutdown signal
			stop()
			return
		}
	}
}

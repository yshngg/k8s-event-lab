package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
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
	klog.InitFlags(flag.CommandLine)
	defer klog.Flush()

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ctx := context.Background()
	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	eventRecorder := eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: common.ComponentEventLab})
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: clientset.CoreV1().Events("")})

	configmap, err := clientset.CoreV1().ConfigMaps(common.ConfigMapNamespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: common.ConfigMapNamespace,
			Name:      common.ConfigMapName,
		},
	}, metav1.CreateOptions{})
	defer clientset.CoreV1().ConfigMaps(common.ConfigMapNamespace).Delete(ctx, "k8s-event-lab", metav1.DeleteOptions{})

	if err != nil {
		klog.Error(err)
		return
	}

	clientset.CoreV1().Events(common.EventNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})

	ticker := time.NewTicker(1 * time.Second)
	i := 0
	for range ticker.C {
		eventRecorder.Event(configmap, common.EventType, common.EventReason, fmt.Sprintf("%s %d", common.EventMessage, i))
		i++
	}
}

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
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/events"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	eventBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: clientset.EventsV1()})

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	eventRecorder := eventBroadcaster.NewRecorder(scheme, common.ComponentEventLab)
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSinkWithContext(ctx)

	configmap, err := clientset.CoreV1().ConfigMaps(common.ConfigMapNamespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: common.ConfigMapNamespace,
			Name:      common.ConfigMapName,
		},
	}, metav1.CreateOptions{})

	if err != nil {
		klog.Error(err)
		stop()
		return
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := clientset.CoreV1().
			ConfigMaps(common.ConfigMapNamespace).
			Delete(ctx, common.ConfigMapName, metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err)
		}
		err = clientset.EventsV1().
			Events(common.EventNamespace).
			DeleteCollection(
				ctx,
				metav1.DeleteOptions{},
				metav1.ListOptions{
					FieldSelector: fmt.Sprintf("involvedObject.uid=%s", configmap.UID),
				})
		if err != nil {
			klog.Error(err)
			return
		}
		klog.Info("clean up residual resources successfully")
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-ticker.C:
			eventRecorder.Eventf(configmap, nil, common.EventType, common.EventReason, common.EventAction, fmt.Sprintf("%s %d", common.EventMessage, i))
			i++
		case <-ctx.Done():
			stop()
			return
		}
	}
}

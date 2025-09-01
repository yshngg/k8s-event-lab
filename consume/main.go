package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/yshngg/k8seventlab/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

	// events, err := clientset.CoreV1().Events("").List(context.Background(), v1.ListOptions{})
	// if err != nil {
	// 	slog.Error("create kubernetes", "err", err)
	// 	os.Exit(1)
	// }
	// for _, event := range events.Items {
	// 	fmt.Println(event.Reason, event.Message)
	// }

	events, err := clientset.CoreV1().Events(common.EventNamespace).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for event := range events.ResultChan() {
		e, ok := event.Object.(*corev1.Event)
		if !ok {
			s := event.Object.(*metav1.Status)
			fmt.Println(s.Code, s.Reason, s.Message, s.Details)
			continue
		}
		switch event.Type {
		case watch.Added, watch.Deleted, watch.Modified:
			if e.Reason == common.EventReason {
				fmt.Println(e.Reason, e.Message, e.FirstTimestamp, e.LastTimestamp, e.Count)
			}
		case watch.Bookmark, watch.Error:
		}
	}
}

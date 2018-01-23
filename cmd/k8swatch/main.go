package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/dbmedialab/rekkoner/velcro/k8s"
)

func main() {
	// Load config; create connection; and auto-discover groups and versions.
	cli, err := k8s.LoadClientConfig(os.Getenv("KUBECONFIG"))
	if err != nil {
		panic(err)
	}

	// Set up watchers and demonstrate change detection on some other resources.
	watchNS, err := cli.Protorequest("Namespace", "").Watch(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	watchDeployments, err := cli.Protorequest("Deployment", "").Watch(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	watchPods, err := cli.Protorequest("Pod", "").Watch(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	watchEvents, err := cli.Protorequest("Event", "").Watch(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for {
		select {
		case evt := <-watchNS.ResultChan():
			printyeEvent("namespace", evt, os.Stdout)
		case evt := <-watchDeployments.ResultChan():
			printyeEvent("deployment", evt, os.Stdout)
		case evt := <-watchPods.ResultChan():
			printyeEvent("pod", evt, os.Stdout)
		case evt := <-watchEvents.ResultChan():
			printyeEvent("event", evt, os.Stdout)
			msg, _ := json.Marshal(evt.Object)
			fmt.Printf("\t%s\n", string(msg))
		}
	}
}

func printyeEvent(label string, evt watch.Event, to io.Writer) {
	fmt.Fprintf(to, ":: evt %-12s %-9v: name=%-65s resourceVersion=%-10s\n",
		label,
		evt.Type,
		evt.Object.(*unstructured.Unstructured).Object["metadata"].(map[string]interface{})["name"],
		evt.Object.(*unstructured.Unstructured).Object["metadata"].(map[string]interface{})["resourceVersion"],
	)
}

func printyeUnstructuredList(label string, list *unstructured.UnstructuredList, to io.Writer) {
	fmt.Fprintf(to, "%s [%s] >>\n", label, list.Object["metadata"].(map[string]interface{})["selfLink"])
	for i, item := range list.Items {
		actualItem := item.Object
		fmt.Printf("\t%.4d -- %#v\n", i, actualItem)
	}
}

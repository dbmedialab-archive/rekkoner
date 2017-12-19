package main

import (
	"flag"
	"fmt"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // Side-effecting import: load the GCP auth plugin, necessary if your kubeconfig references that.
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func main() {
	var kubeconfig string
	var master string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	// creates the connection
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: master}}).ClientConfig()
	if err != nil {
		panic(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// Please
	config.GroupVersion = &schema.GroupVersion{Version: "v1"}
	config.APIPath = "/api"
	dyn, err := dynamic.NewClient(config)
	if err != nil {
		panic(err)
	}
	obj, err := dyn.Resource(&meta_v1.APIResource{Name: "namespaces"}, "").List(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("WOOW::\n\t%#v\n::\n", obj)

	config.APIPath = "/apis"
	config.GroupVersion = &schema.GroupVersion{Version: "extensions/v1beta1"}
	dyn, err = dynamic.NewClient(config)
	if err != nil {
		panic(err)
	}
	obj, err = dyn.Resource(&meta_v1.APIResource{Name: "deployments"}, "default").List(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("WOOW::\n\t%#v\n::\n", obj)

	// Okay, trying to figure out what this cacher thing does and I ended up having
	// to read a `NewNamedReflector` factory and *still* haven't found any meat...
	// fuck this.  This is the worst architecture astronauting I've ever seen.
	// I literally can't find any logic that *does* *any* *thing*.

	watchFunc := func(c rest.Interface, resource string, namespace string, fieldSelector fields.Selector) (watch.Interface, error) {
		options := meta_v1.ListOptions{}
		options.Watch = true
		options.FieldSelector = fieldSelector.String()
		return c.Get().
			Namespace(namespace).
			Resource(resource).
			VersionedParams(&options, meta_v1.ParameterCodec).
			Watch() // I think I'm actually pretty ok with using their API up to right about here.
		// nope it still already did the hypermagic typed deserialize
		// I really don't want that
		// it is so fucking much easier to iterate over *maps* to do this job
		// i'm literally going to back ALLLLL the way up off to doing my own rest client.
	}
	watchNS, err := watchFunc(clientset.CoreV1().RESTClient(), "namespaces", "", fields.Everything())
	if err != nil {
		panic(err)
	}
	watchPods, err := watchFunc(clientset.CoreV1().RESTClient(), "pods", "", fields.Everything())
	if err != nil {
		panic(err)
	}
	for {
		select {
		case evt := <-watchNS.ResultChan():
			fmt.Printf(":: evt %T %#v\n\n", evt, evt)
		case evt := <-watchPods.ResultChan():
			fmt.Printf(":: evt %T %#v\n\n", evt, evt)
		}
	}
}

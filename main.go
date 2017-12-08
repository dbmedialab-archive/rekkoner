package main

import (
	"flag"
	"fmt"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var kubeconfig string
	var master string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	// creates the connection
	// TODO please replace this with something with more self-control, this tries way
	//  too hard to be helpful and add a bunch of flags through globals and... no.
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		panic(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

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

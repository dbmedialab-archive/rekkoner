package main

import (
	"flag"
	"fmt"

	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
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

	return

	// create the pod watcher
	podListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "namespaces", "", fields.Everything())

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the workqueue to a cache with the help of an informer. This way we make sure that
	// whenever the cache is updated, the pod key is added to the workqueue.
	// Note that when we finally process the item from the workqueue, we might see a newer version
	// of the Pod than the version which was responsible for triggering the update.
	//
	// There's a requirement that whatever you use as the type here implement
	// k8s.io/apimachinery/pkg/runtime.Object ... so we do that.
	indexer, informer := cache.NewIndexerInformer(podListWatcher, runtime.Object(nil), 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj) // FIXME i have no idea why they don't use UUID here
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	_, _, _ = queue, indexer, informer

	controller := NewController(queue, indexer, informer)

	// We can now warm up the cache for initial synchronization.
	// Let's suppose that we knew about a pod "mypod" on our last run, therefore add it to the cache.
	// If this pod is not there anymore, the controller will be notified about the removal after the
	// cache has synchronized.
	if err := indexer.Add(&v1.Node{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "hahaha",
			Namespace: v1.NamespaceDefault,
		},
	}); err != nil {
		panic(err)
	}

	// Now let's start the controller
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	// Wait forever
	select {}
}

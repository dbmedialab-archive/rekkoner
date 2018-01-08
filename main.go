package main

import (
	"fmt"
	"io"
	"os"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // Side-effecting import: load the GCP auth plugin, necessary if your kubeconfig references that.
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// creates the connection
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: os.Getenv("KUBECONFIG")},
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		panic(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// Figuring out groups, versions, etc:
	//
	// I'd kind of like to *not* have to explicitly specify e.g. "extensions/v1beta1/deployments"
	// or similar pattern of string for *every single* resource I need *multiplied* by wherever
	// those versions are on the particular version of k8s master I'm talking to.
	//
	// So how do we auto-detect that?
	//
	// - get 'servergroups.json'
	// - iterate over '.groups' list
	// - take '.name' and '.preferredVersion.version' -- plug them into your `schema.GroupVersion`, woot
	//   - n.b. I *think* strcat'ing those is identical to '.preferredVersion.groupVersion' but ¯\_(ツ)_/¯
	// - ok, but *what* is that the group+version tuple **for**???  well we don't know yet
	// - get '$group/$version/serverresources.json'... for all group+version tuples I guess
	//   - there's apparently no index that lets us find out which kinds will be where... so, yep, all
	//   - so yes, you need to *do $n$ https requests* to *find out what you can ask for*, nbd
	//     - this explains why `~/.kube/cache/discovery/*` probably exists in your homedir... -.-
	//       - btw note well none of these have generation numbers or anything you could cachebust with either
	// - iterate through this and look for '.name'
	//   - THIS you can finally match on!  look for e.g. "deployments" here and you should find it.
	// - and you may want to look for the '.namespaced' bool here as well (but your business
	//   logic probably already know that one, in practice).
	//
	// Implementation: ugh, todo.
	// (SURELY this should exist in the k8s client packages already.  But can I get at
	// it in a way that I can A: cache more sanely, and B: compose with the `dynamic.NewClient` route
	// so that it's literally even fit for use?  Based on prior experiences: less than 50% chance.)

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
	printyeUnstructuredList("namespaces", obj.(*unstructured.UnstructuredList), os.Stdout)

	config.APIPath = "/apis"
	config.GroupVersion = &schema.GroupVersion{Group: "extensions", Version: "v1beta1"}
	dyn, err = dynamic.NewClient(config)
	if err != nil {
		panic(err)
	}
	obj, err = dyn.Resource(&meta_v1.APIResource{Name: "deployments"}, "default").List(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	printyeUnstructuredList("deployments", obj.(*unstructured.UnstructuredList), os.Stdout)

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

func printyeUnstructuredList(label string, list *unstructured.UnstructuredList, to io.Writer) {
	fmt.Fprintf(to, "%s [%s] >>\n", label, list.Object["metadata"].(map[string]interface{})["selfLink"])
	for i, item := range list.Items {
		actualItem := item.Object
		fmt.Printf("\t%.4d -- %#v\n", i, actualItem)
	}
}

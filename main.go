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
	"k8s.io/client-go/discovery"
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
	resources, err := saneDiscovery(config)
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
	printyeUnstructuredList("namespaces", obj.(*unstructured.UnstructuredList), os.Stdout)

	config.APIPath = "/apis"
	fmt.Printf("\n:resource yo %#v\n\n", resources["Deployment"])
	config.GroupVersion = &schema.GroupVersion{Group: resources["Deployment"].Group, Version: resources["Deployment"].Version}
	dyn, err = dynamic.NewClient(config)
	if err != nil {
		panic(err)
	}
	obj, err = dyn.Resource(&meta_v1.APIResource{Name: resources["Deployment"].Name}, "default").List(meta_v1.ListOptions{})
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

// This function wraps k8s `discovery.Client` and makes it actually return sane,
// usable data:
//
//   - it massages the results so "group" and "version" are actually *in*
//     the `APIResource` objects, instead of in a weird place off to one side
//     that's a huge PITA to pass around;
//   - it returns things in a map, indexed by the actual Kind name, instead
//     of making you continue to fumble around looking for the one thing you
//     probably wanted this entire time;
//   - it doesn't bother you with anything except the server's preferredVersions;
//   - and if the server has the same "Kind" listed under more than one group,
//     welp, that's funny.  Last one in the list wins.
//
// tl;dr probably what you wanted 99.9999999% of the time.
// And the other 0.0000001% of the time, you're writing a discovery client printer.
//
// NOTE: I said indexed by the "Kind".  So, e.g., "Deployment", title-case.
// The "Kind" seems to be the most consistently used string across the project.
// The API route slug is typically different cased, and inflected to be plural --
// this is just to make you squirm, as far as I can figure out.
// Fortunately, the inflected API route slugs can be read in the APIResource objects.
//
// NOTE: I feel *bad* about that cavalier comment about the same Kind name existing
// under multiple groups being a problem, but *it's true*.  It's *very* unfortunate,
// since the point of "groups" was for some namespacing here, but *critical* parts
// of the k8s ecosystem -- namely, the "Deployment" kind -- has been jumping around
// between different groups for several releases, and uhm, we kind of need to track
// that one.  It's only like... the linchpin of almost all production services.  NBD.
//
// NOTE FURTHER: Despite the discovery client's API returning a *list*, it's *not
// consistently ordered*, so collisions will have overall nondeterministic results.
// I don't know if it's the discovery client or the remote API itself being chaotic.
//
// NOTE: Despite all our bests efforts to sanitize this process, it still wastes
// about 1.5 seconds *wall clock* on network RTTs for the auto-discover.
// I think it's *horrific* if a tool like this can't run *without* disk write, but
// some sort of cache really might be a practical necessity given the deck we've
// been dealt with this discovery API.
//
func saneDiscovery(c *rest.Config) (map[string]meta_v1.APIResource, error) {
	discoClient, err := discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		return nil, err
	}
	groupedResources, err := discoClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}
	// This is a list of... lists... why
	indexedResources := make(map[string]meta_v1.APIResource)
	for _, resourceGroup := range groupedResources {
		// Parse the 'GroupVersion' field back apart... because god forbid anyone
		// on some sort of design committee choose whether that's consistently
		// supposed to be one conjoined or two strings across the entire project.
		groupVersion, err := schema.ParseGroupVersion(resourceGroup.GroupVersion)
		if err != nil {
			panic(err) // protocol violation, do not want.
		}

		// Range again over the actual *resources* in the group.
		// You'd think we'd already have things normalized to that form, since we
		// asked for `ServerPreferredResources` above, but alas, no.
		for _, resource := range resourceGroup.APIResources {
			// Set the group and version to true values.
			// Again, consistently in our inconsistency here in k8s land, this
			// actually uses two string fields *separately*, rather than *either*
			// the conjoined string we just got, nor the schema.GroupVersion tuple
			// which is its most canonical representation and used through most
			// of the rest of the client APIs.  YOLO; why bother making choices?
			resource.Group = groupVersion.Group
			resource.Version = groupVersion.Version

			// Index.
			indexedResources[resource.Kind] = resource
		}
	}
	return indexedResources, nil
}

func printyeUnstructuredList(label string, list *unstructured.UnstructuredList, to io.Writer) {
	fmt.Fprintf(to, "%s [%s] >>\n", label, list.Object["metadata"].(map[string]interface{})["selfLink"])
	for i, item := range list.Items {
		actualItem := item.Object
		fmt.Printf("\t%.4d -- %#v\n", i, actualItem)
	}
}

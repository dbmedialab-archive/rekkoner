package main

import (
	"fmt"
	"io"
	"os"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
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

	// Figuring out groups, versions, etc:
	resources, err := saneDiscovery(config)
	if err != nil {
		panic(err)
	}

	// List a snapshot of state for some various resources.
	obj, err := listDynamically(config, resources, "Namespace", "").List(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	printyeUnstructuredList("namespaces", obj.(*unstructured.UnstructuredList), os.Stdout)

	obj, err = listDynamically(config, resources, "Deployment", "default").List(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	printyeUnstructuredList("deployments", obj.(*unstructured.UnstructuredList), os.Stdout)

	// Set up watchers and demonstrate change detection on some other resources.

	watchNS, err := listDynamically(config, resources, "Namespace", "").Watch(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	watchPods, err := listDynamically(config, resources, "Pod", "").Watch(meta_v1.ListOptions{})
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

// TODO we probably want to bundle the rest.Config and the cache (also indexed, ffs) of APIResource info into one object.

func listDynamically(c *rest.Config, allThings map[string]meta_v1.APIResource, theThing string, namespace string) dynamic.ResourceInterface {
	// Pick out the relevant APIResource info.
	resourceDesc, ok := allThings[theThing]
	if !ok {
		panic(fmt.Errorf("no resource called %q available on this server", theThing))
	}

	// Massage the REST config first.
	//
	// I'd do a DeepCopy on this so we don't mutate the object from our caller, but
	// *that isn't actually possible* because that returns a completely different
	// type!  So, yeah, that method is both worse than useless and absolutely
	// does not do what it says on the tin.
	//
	// This dereference-put-on-stack-and-grab-new-reference hijink does a copy.
	// It's the same thing `dynamic.NewClient` does, so it must be fine.
	confCopy := *c
	c = &confCopy
	// Specify an API path.
	//
	// Surely this can't be that complicated, you say!  Well...
	//
	// OBVIOUSLY, this can be two different paths, right at the base.
	// OBVIOUSLY, this was necessary to separate the grouped API extensions...
	// despite the fact *the groups themselves do that*.
	// OBVIOUSLY, the clear way to handle this isn't to migrate everything to groups;
	// OBVIOUSLY, we should leave the "core" "legacy" APIs on a separate base path;
	// OBVIOUSLY, we should indicate this by letting you parse the GroupVersion string
	// and checking if it has a slash in it (obviously indicating lack of group),
	// and OBVIOUSLY, separating things by "/api/" vs "/apis/" will be VERY clear,
	// since it's not like we used inflected pluralizations anywhere else in the API
	// (oh wait literally everywhere).
	//
	// All this is documented at https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-groups
	if resourceDesc.Group == "" {
		c.APIPath = "/api"
	} else {
		c.APIPath = "/apis"
	}
	// This may look odd and redundant, because these fields are in the APIResource
	// object that we're about to give to the `Resource` method anyway, but they're
	// not actually used by that method (god forbid the arguments give you a hint
	// at what a method will *do*, apparently), so we must set them here.
	//
	// (And why Yes, astute reader, we *did* just have to separate the `GroupVersion`
	// struct into separate `Group` and `Version` strings just moments ago, to
	// fabricate this very `APIResource` object which we are now taking apart to...
	// yes, build a `GroupVersion` object again.  Whee!  Consistency!)
	c.GroupVersion = &schema.GroupVersion{Group: resourceDesc.Group, Version: resourceDesc.Version}
	// This is the only line of this function which is (arguably) not a joke.
	dyn, err := dynamic.NewClient(c)
	// Audit of 'rest.RESTClientFor' indicates this error is only possible from
	// invalid configuration (e.g., we're not doing any network action yet, thank
	// deity), so, we'll treat it as a panick-worthy offence.
	if err != nil {
		panic(err)
	}

	// We *do not* care about the ratelimiter, because it's a half-baked idea and with no semantic QOS can cause ugly disordered starvation issues.
	// We *do not* care about setting a ParameterCodec.  We went through *all* this work to get as close to bare, iterable maps and lists as possible.
	// That leaves nothing but the `Resource` method.  Meaning... we don't carea bout the `dynamic.Client` type *at all*.
	// And thus we shan't return one.  We'll give you the `ResourceInterface` already built.
	return dyn.Resource(&resourceDesc, namespace)
}

func printyeUnstructuredList(label string, list *unstructured.UnstructuredList, to io.Writer) {
	fmt.Fprintf(to, "%s [%s] >>\n", label, list.Object["metadata"].(map[string]interface{})["selfLink"])
	for i, item := range list.Items {
		actualItem := item.Object
		fmt.Printf("\t%.4d -- %#v\n", i, actualItem)
	}
}

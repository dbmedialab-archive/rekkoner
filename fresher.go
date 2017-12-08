package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// so we want this for... i'm good with one namespace (more requires more connections no matter what iiuc)
// but we want it for more than one resource... and do we give a dang about the field selector?  is it per resource?
func NewListWatchFromClient(c rest.RESTClient, resource string, namespace string, fieldSelector fields.Selector) cache.ListerWatcher {
	listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
		options.FieldSelector = fieldSelector.String()
		return c.Get().
			Namespace(namespace).
			Resource(resource).
			VersionedParams(&options, metav1.ParameterCodec).
			Do().
			Get()
	}
	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		options.Watch = true
		options.FieldSelector = fieldSelector.String()
		return c.Get().
			Namespace(namespace).
			Resource(resource).
			VersionedParams(&options, metav1.ParameterCodec).
			Watch() // I think I'm actually pretty ok with using their API up to right about here.
	}
	_, _ = listFunc, watchFunc
	// TODO we still haven't figured out *where the reifier fires its lasers*
	return nil // TODO finish
}

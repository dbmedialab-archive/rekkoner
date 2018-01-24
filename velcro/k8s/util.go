package k8s

import "k8s.io/apimachinery/pkg/runtime"

// Unwrap a `k8s.UnstructuredList` into `[]k8s.Unstructured`.
// Takes an `interface{}` wildcard because often the k8s API
// will actually just hand you a `runtime.Object`, which is kind
// of irritating.
func UnwrapList(listish runtime.Object) []Unstructured {
	return listish.(*UnstructuredList).Items
}

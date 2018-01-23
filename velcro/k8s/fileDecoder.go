package k8s

import (
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func DecodeObjects(r io.Reader) ([]unstructured.Unstructured, error) {
	d := yaml.NewYAMLOrJSONDecoder(r, 4096)
	objs := []unstructured.Unstructured{}
	for { // loop because yaml can contain multiple objects
		slot := unstructured.Unstructured{}
		if err := d.Decode(&slot); err != nil {
			if err == io.EOF {
				return objs, nil
			}
			return objs, err
		}
		objs = append(objs, slot)
	}
}

package k8s

import (
	"bytes"
	"testing"
)

func TestFileDecoding(t *testing.T) {
	f := bytes.NewBufferString(`apiVersion: v1
kind: Namespace
metadata:
  name: "{{env \"WERCKER_GIT_REPOSITORY\"}}"
  labels:
    environment: "{{env \"ENV\"}}"
`)
	t.Logf("parsed as: %#v\n", DecodeObjects(f))
}

package main

import (
	"testing"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
)

var input = []byte(string(`
apiVersion: v1
kind: Pod
metadata:
  name: foo
`))

func unmarshalFromYamlForCodecs(buffer []byte, gv schema.GroupVersion, codecs serializer.CodecFactory) (runtime.Object, error) {
	const mediaType = runtime.ContentTypeYAML
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		return nil, errors.Errorf("unsupported media type %q", mediaType)
	}

	decoder := codecs.DecoderToVersion(info.Serializer, gv)
	obj, err := runtime.Decode(decoder, buffer)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode %s into runtime.Object", buffer)
	}
	return obj, nil
}

func TestFunc(t *testing.T) {
	obj, err := unmarshalFromYamlForCodecs(input, v1.SchemeGroupVersion, clientsetscheme.Codecs)
	if err != nil {
		t.Fatalf("error: %v\n", err)
	}

	pod := obj.(*v1.Pod)
	if pod.Spec.DNSPolicy != "" { // simply check if one of the values we care about was defaulted from ""
		t.Fatal("error")
	}
}

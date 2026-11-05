/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package incubating

import (
	"context"
	"fmt"
	"strings"
	"testing"

	resourceapi "k8s.io/api/resource/v1"
	draapi "k8s.io/dynamic-resource-allocation/api"
	"k8s.io/dynamic-resource-allocation/cel"
)

// deviceProgKey is the evaluation-cache key suggested in
// https://github.com/kubernetes/kubernetes/pull/140029#discussion_r3636503040 :
// DeviceID plus the pinned compilation result, instead of the CEL text.
type deviceProgKey struct {
	DeviceID
	prog *cel.CompilationResult
}

var cacheBenchSink any

func BenchmarkDerivedAttributeEvalCache(b *testing.B) {
	const numDevices = 512
	shortExpr := `device.attributes["resource.kubernetes.io"].pciRoot`
	longExpr := longDerivedAttributeExpr()

	for _, tc := range []struct {
		name string
		expr string
	}{
		{name: "short", expr: shortExpr},
		{name: "long", expr: longExpr},
	} {
		b.Run(tc.name, func(b *testing.B) {
			ids := benchDeviceIDs(numDevices)
			compiled := compileDerived(b, tc.expr)

			b.Run("string-key", func(b *testing.B) {
				cache := make(map[deviceExprKey]any, len(ids))
				for _, id := range ids {
					cache[deviceExprKey{DeviceID: id, expr: tc.expr}] = "pci0000:00"
				}
				b.ResetTimer()
				for i := 0; b.Loop(); i++ {
					cacheBenchSink = cache[deviceExprKey{DeviceID: ids[i%len(ids)], expr: tc.expr}]
				}
			})

			b.Run("pointer-key", func(b *testing.B) {
				cache := make(map[deviceProgKey]any, len(ids))
				for _, id := range ids {
					cache[deviceProgKey{DeviceID: id, prog: compiled}] = "pci0000:00"
				}
				b.ResetTimer()
				for i := 0; b.Loop(); i++ {
					cacheBenchSink = cache[deviceProgKey{DeviceID: ids[i%len(ids)], prog: compiled}]
				}
			})

			b.Run("lookupAttribute-hit", func(b *testing.B) {
				alloc, request, device, attr := benchLookup(b, tc.expr, ids)
				b.ResetTimer()
				for i := 0; b.Loop(); i++ {
					val, err := alloc.lookupAttribute(request, device, ids[i%len(ids)], attr)
					if err != nil {
						b.Fatal(err)
					}
					cacheBenchSink = val
				}
			})
		})
	}
}

func benchLookup(b *testing.B, expr string, ids []DeviceID) (*allocator, *requestData, *draapi.Device, draapi.FullyQualifiedName) {
	b.Helper()
	alloc := &allocator{
		Allocator: &Allocator{
			celCache: cel.NewCache(10, cel.Features{}),
		},
		ctx:                    context.Background(),
		derivedAttributesCache: make(map[deviceExprKey]any),
	}
	attr := draapi.FullyQualifiedName{
		Domain:     draapi.MakeUniqueString("test-driver.cdi.k8s.io"),
		Identifier: draapi.MakeUniqueString("derivedPciRoot"),
	}
	request := &requestData{
		derivedAttributes: map[resourceapi.FullyQualifiedName]string{
			resourceapi.FullyQualifiedName(attr.String()): expr,
		},
	}
	device := benchDevice()
	for _, id := range ids {
		if _, err := alloc.lookupAttribute(request, device, id, attr); err != nil {
			b.Fatal(err)
		}
	}
	return alloc, request, device, attr
}

func compileDerived(b *testing.B, expr string) *cel.CompilationResult {
	b.Helper()
	res := cel.NewCache(1, cel.Features{}).GetOrCompileDerivedAttribute(expr)
	if res.Error != nil {
		b.Fatal(res.Error)
	}
	return &res
}

func benchDevice() *draapi.Device {
	domain := draapi.MakeUniqueString("resource.kubernetes.io")
	id := draapi.MakeUniqueString("pciRoot")
	return &draapi.Device{
		Attributes: draapi.DeviceAttributes{
			Nested: map[draapi.UniqueString]map[draapi.UniqueString]any{
				domain: {
					id: "pci0000:00",
				},
			},
		},
	}
}

func benchDeviceIDs(n int) []DeviceID {
	ids := make([]DeviceID, n)
	for i := range ids {
		ids[i] = MakeDeviceID("test-driver.cdi.k8s.io", "pool", fmt.Sprintf("dev-%d", i))
	}
	return ids
}

func longDerivedAttributeExpr() string {
	const maxLen = 10 * 1024
	prefix := `device.attributes["resource.kubernetes.io"].pciRoot + "`
	suffix := `"`
	return prefix + strings.Repeat("x", maxLen-len(prefix)-len(suffix)) + suffix
}

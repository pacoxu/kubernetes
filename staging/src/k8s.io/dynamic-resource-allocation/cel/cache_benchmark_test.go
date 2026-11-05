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

package cel

import (
	"fmt"
	"strings"
	"testing"
)

// schedulerCELCacheEntries is the cache size used by the scheduler
// (dynamicresources plugin and scheduler_perf).
const schedulerCELCacheEntries = 10

// deviceDerivedAttributesMaxSize is resource.DeviceDerivedAttributesMaxSize.
// Duplicated here so this benchmark does not import k8s.io/kubernetes.
const deviceDerivedAttributesMaxSize = 32

var benchSink any

func BenchmarkDerivedAttributeCompileCache(b *testing.B) {
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
			b.Run("lru-hit", func(b *testing.B) {
				cache := NewCache(schedulerCELCacheEntries, Features{})
				if res := cache.GetOrCompileDerivedAttribute(tc.expr); res.Error != nil {
					b.Fatal(res.Error)
				}
				b.ResetTimer()
				for b.Loop() {
					res := cache.GetOrCompileDerivedAttribute(tc.expr)
					if res.Error != nil {
						b.Fatal(res.Error)
					}
					benchSink = res.Program
				}
			})

			b.Run("pinned", func(b *testing.B) {
				cache := NewCache(schedulerCELCacheEntries, Features{})
				res := cache.GetOrCompileDerivedAttribute(tc.expr)
				if res.Error != nil {
					b.Fatal(res.Error)
				}
				// Hold the compilation result for the whole benchmark. The
				// LRU cache does not promise this: it only stores 10 entries
				// and drops the oldest.
				pinned := &res
				b.ResetTimer()
				for b.Loop() {
					benchSink = pinned.Program
				}
			})
		})
	}

	// 32 distinct expressions do not fit in the scheduler's 10-entry LRU,
	// so each lookup recompiles. The pinned slice keeps every result.
	exprs := distinctDerivedAttributeExprs(deviceDerivedAttributesMaxSize)
	b.Run("thirty-two-exprs", func(b *testing.B) {
		b.Run("lru-size-10", func(b *testing.B) {
			cache := NewCache(schedulerCELCacheEntries, Features{})
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				res := cache.GetOrCompileDerivedAttribute(exprs[i%len(exprs)])
				if res.Error != nil {
					b.Fatal(res.Error)
				}
				benchSink = res.Program
			}
		})

		b.Run("pinned", func(b *testing.B) {
			cache := NewCache(len(exprs), Features{})
			pinned := make([]*CompilationResult, len(exprs))
			for i, expr := range exprs {
				res := cache.GetOrCompileDerivedAttribute(expr)
				if res.Error != nil {
					b.Fatal(res.Error)
				}
				pinned[i] = &res
			}
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				benchSink = pinned[i%len(pinned)].Program
			}
		})
	})
}

func longDerivedAttributeExpr() string {
	// Just under CELSelectorExpressionMaxLength (10 Ki). The padding is a
	// string literal, so the expression still type-checks as a string.
	const maxLen = 10 * 1024
	prefix := `device.attributes["resource.kubernetes.io"].pciRoot + "`
	suffix := `"`
	return prefix + strings.Repeat("x", maxLen-len(prefix)-len(suffix)) + suffix
}

func distinctDerivedAttributeExprs(n int) []string {
	exprs := make([]string, n)
	for i := range exprs {
		exprs[i] = fmt.Sprintf(`device.attributes["resource.kubernetes.io"].pciRoot + "%04d"`, i)
	}
	return exprs
}

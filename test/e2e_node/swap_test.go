/*
Copyright 2019 The Kubernetes Authors.

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

package e2enode

import (
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefeatures "k8s.io/kubernetes/pkg/features"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	reservedSwapSize = "256Mi"
)

// Serial because the test updates kubelet configuration.
var _ = SIGDescribe("System reserved swap [Serial]", func() {
	f := framework.NewDefaultFramework("system-reserved swap test")
	ginkgo.Context("With config updated with swap reserved", func() {
		if !isSwapOn() {
			ginkgo.Skip("skipping test when swap is not open on host")
		}
		tempSetCurrentKubeletConfig(f, func(initialConfig *kubeletconfig.KubeletConfiguration) {
			initialConfig.FailSwapOn = false
			initialConfig.FeatureGates[string(kubefeatures.NodeSwap)] = true
			initialConfig.MemorySwap = kubeletconfig.MemorySwapConfiguration{
				SwapBehavior: "LimitedSwap",
			}
			initialConfig.SystemReserved[string(v1.ResourceSwap)] = reservedSwapSize
		})
		runSystemReservedSwapTests(f)
	})
})

func runSystemReservedSwapTests(f *framework.Framework) {
	ginkgo.It("node should not allocate reserved swap size", func() {
		ginkgo.By("by check node status")
		nodeList, err := f.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		framework.ExpectNoError(err)
		// Assuming that there is only one node, because this is a node e2e test.
		framework.ExpectEqual(len(nodeList.Items), 1)
		allocateble := nodeList.Items[0].Status.Allocatable.Swap()
		capacity := nodeList.Items[0].Status.Capacity.Swap()
		reserved := resource.MustParse(reservedSwapSize)
		reserved.Add(*allocateble)
		framework.ExpectEqual(reserved.Cmp(*capacity), 0)
	})
}

func isSwapOn() bool {
	var output strings.Builder
	cmd := exec.Command("/bin/sh", "-c", "cat /proc/meminfo|grep 'SwapTotal'|awk '{print$2}'")
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return false
	}
	num, err := strconv.Atoi(output.String())
	if err != nil {
		return false
	}
	return num > 0
}

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
	"fmt"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefeatures "k8s.io/kubernetes/pkg/features"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	// Dir to enable swap.
	swapDir          = "/mnt/swap"
	totalSwapSize    = 512 // 512Mi
	reservedSwapSize = "256Mi"
)

// Serial because the test updates kubelet configuration.
var _ = SIGDescribe("System reserved swap [Serial]", func() {
	f := framework.NewDefaultFramework("system-reserved swap test")
	ginkgo.Context("With config updated with swap reserved", func() {
		// setup
		ginkgo.JustBeforeEach(func() {
			gomega.Eventually(func() error {
				return setupSwap(swapDir)
			}, 30*time.Second, framework.Poll).Should(gomega.BeNil())
		})
		// cleanup
		ginkgo.JustAfterEach(func() {
			gomega.Eventually(func() error {
				return cleanupSwap(swapDir)
			}, 30*time.Second, framework.Poll).Should(gomega.BeNil())
		})
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
		// TODO (jonyhy96) use Swap in ResourceList when supported
		allocateble := nodeList.Items[0].Status.Allocatable[v1.ResourceSwap]
		capacity := nodeList.Items[0].Status.Capacity[v1.ResourceSwap]
		reserved := resource.MustParse(reservedSwapSize)
		reserved.Add(allocateble)
		framework.ExpectEqual(reserved.Cmp(capacity), 0)
	})
}

func setupSwap(path string) error {
	// will allocate 512M on given path
	newDirCommand := fmt.Sprintf("dd if=/dev/zero of=%s bs=1M count=%d", path, totalSwapSize)
	mkswapCommand := fmt.Sprintf("mkswap %s", path)
	swapOnCommand := fmt.Sprintf("swapon %s", path)
	if err := exec.Command("/bin/sh", "-c",
		fmt.Sprintf("%s && %s && %s", newDirCommand, mkswapCommand, swapOnCommand)).Run(); err != nil {
		return err
	}
	return nil
}

func cleanupSwap(path string) error {
	if err := exec.Command("/bin/sh", "-c",
		"swapoff %s && rm -rf %s", path, path).Run(); err != nil {
		return err
	}
	return nil
}

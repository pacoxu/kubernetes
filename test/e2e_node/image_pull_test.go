/*
Copyright 2024 The Kubernetes Authors.

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
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	kubeletevents "k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/nodefeature"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

// This test needs to run in serial to prevent caching of the images by other tests
// and to prevent the wait time of image pulls to be increased by other images
var _ = SIGDescribe("Pull Image", framework.WithSerial(), nodefeature.MaxParallelImagePull, func() {

	f := framework.NewDefaultFramework("parallel-pull-image-test")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
	var testpods []*v1.Pod

	ginkgo.Context("parallel image pull with MaxParallelImagePulls=5", func() {
		tempSetCurrentKubeletConfig(f, func(ctx context.Context, initialConfig *kubeletconfig.KubeletConfiguration) {
			initialConfig.SerializeImagePulls = false
			initialConfig.MaxParallelImagePulls = ptr.To[int32](5)
		})

		ginkgo.BeforeEach(func(ctx context.Context) {
			testpods = prepareAndCleanup(ctx, f)
		})

		ginkgo.AfterEach(func(ctx context.Context) {
			ginkgo.By("cleanup pods")
			for _, pod := range testpods {
				deletePodSyncByName(ctx, f, pod.Name)
			}
		})

		ginkgo.It("should pull immediately if no more than 5 pods", func(ctx context.Context) {
			for _, testpod := range testpods {
				pod := e2epod.NewPodClient(f).Create(ctx, testpod)
				err := e2epod.WaitForPodCondition(ctx, f.ClientSet, f.Namespace.Name, pod.Name, "Running", 30*time.Second, func(pod *v1.Pod) (bool, error) {
					if pod.Status.Phase == v1.PodRunning {
						return true, nil
					}
					return false, nil
				})
				framework.ExpectNoError(err)
			}

			events, err := f.ClientSet.CoreV1().Events(f.Namespace.Name).List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err)
			var httpdPulled []*pulledStruct
			for _, event := range events.Items {
				if event.Reason == kubeletevents.PulledImage {
					for _, testpod := range testpods {
						if event.InvolvedObject.Name == testpod.Name {
							pulled, err := getDurationsFromPulledEventMsg(event.Message)
							httpdPulled = append(httpdPulled, pulled)
							framework.ExpectNoError(err)
							break
						}
					}
				}
			}
			gomega.Expect(len(testpods)).To(gomega.BeComparableTo(len(httpdPulled)))

			// as this is parallel image pulling, the waiting duration should be similar with the pulled duration.
			// use 1.2 as a common ratio
			for _, pulled := range httpdPulled {
				if float32(pulled.pulledIncludeWaitingDuration/time.Millisecond)/float32(pulled.pulledDuration/time.Millisecond) > 1.2 {
					framework.Failf("the pull duration including waiting %v should be similar with the pulled duration %v",
						pulled.pulledIncludeWaitingDuration, pulled.pulledDuration)
				}
			}
		})

	})
})

var _ = SIGDescribe("Pull Image", framework.WithSerial(), nodefeature.MaxParallelImagePull, func() {

	f := framework.NewDefaultFramework("serialize-pull-image-test")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	ginkgo.Context("serialize image pull", func() {
		// this is the default behavior now.
		tempSetCurrentKubeletConfig(f, func(ctx context.Context, initialConfig *kubeletconfig.KubeletConfiguration) {
			initialConfig.SerializeImagePulls = true
			initialConfig.MaxParallelImagePulls = ptr.To[int32](1)
		})

		var testpods []*v1.Pod

		ginkgo.BeforeEach(func(ctx context.Context) {
			testpods = prepareAndCleanup(ctx, f)
		})

		ginkgo.AfterEach(func(ctx context.Context) {
			ginkgo.By("cleanup pods")
			for _, pod := range testpods {
				deletePodSyncByName(ctx, f, pod.Name)
			}
		})

		ginkgo.It("should be waiting more", func(ctx context.Context) {
			for _, testpod := range testpods {
				pod := e2epod.NewPodClient(f).Create(ctx, testpod)
				err := e2epod.WaitForPodCondition(ctx, f.ClientSet, f.Namespace.Name, pod.Name, "Running", 30*time.Second, func(pod *v1.Pod) (bool, error) {
					if pod.Status.Phase == v1.PodRunning {
						return true, nil
					}
					return false, nil
				})
				framework.ExpectNoError(err)
			}

			events, err := f.ClientSet.CoreV1().Events(f.Namespace.Name).List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err)
			var httpdPulled []*pulledStruct
			for _, event := range events.Items {
				if event.Reason == kubeletevents.PulledImage {
					for _, testpod := range testpods {
						if event.InvolvedObject.Name == testpod.Name {
							pulled, err := getDurationsFromPulledEventMsg(event.Message)
							httpdPulled = append(httpdPulled, pulled)
							framework.ExpectNoError(err)
							break
						}
					}
				}
			}
			gomega.Expect(len(testpods)).To(gomega.BeComparableTo(len(httpdPulled)))

			// as this is serialize image pulling, the waiting duration should be almost double the duration with the pulled duration.
			// use 1.5 as a common ratio to avoid some overlap during pod creation
			if float32(httpdPulled[1].pulledIncludeWaitingDuration/time.Millisecond)/float32(httpdPulled[1].pulledDuration/time.Millisecond) < 1.5 &&
				float32(httpdPulled[0].pulledIncludeWaitingDuration/time.Millisecond)/float32(httpdPulled[0].pulledDuration/time.Millisecond) < 1.5 {
				framework.Failf("At least, one of the pull duration including waiting %v/%v should be similar with the pulled duration %v/%v",
					httpdPulled[1].pulledIncludeWaitingDuration, httpdPulled[0].pulledIncludeWaitingDuration, httpdPulled[1].pulledDuration, httpdPulled[0].pulledDuration)
			}
		})

	})
})

func prepareAndCleanup(ctx context.Context, f *framework.Framework) (testpods []*v1.Pod) {
	httpdImage := imageutils.GetE2EImage(imageutils.Httpd)
	httpdNewImage := imageutils.GetE2EImage(imageutils.HttpdNew)
	node := getNodeName(ctx, f)

	testpod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "httpd",
			Namespace: f.Namespace.Name,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:            "httpd",
				Image:           httpdImage,
				ImagePullPolicy: v1.PullAlways,
			}},
			NodeName:      node,
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
	testpod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "httpd2",
			Namespace: f.Namespace.Name,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:            "httpd-new",
				Image:           httpdNewImage,
				ImagePullPolicy: v1.PullAlways,
			}},
			NodeName:      node,
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
	testpods = []*v1.Pod{testpod, testpod2}

	ginkgo.By("cleanup images")
	for _, pod := range testpods {
		_ = RemoveImage(ctx, pod.Spec.Containers[0].Image)
	}
	return testpods
}

type pulledStruct struct {
	pulledDuration               time.Duration
	pulledIncludeWaitingDuration time.Duration
}

// getDurationsFromPulledEventMsg will parse two durations in the pulled message
// Example msg: `Successfully pulled image \"busybox:1.28\" in 39.356s (49.356s including waiting). Image size: 41901587 bytes.`
func getDurationsFromPulledEventMsg(msg string) (*pulledStruct, error) {
	splits := strings.Split(msg, " ")
	if len(splits) != 13 {
		return nil, errors.Errorf("pull event message should be spilted to 13: %d", len(splits))
	}
	pulledDuration, err := time.ParseDuration(splits[5])
	if err != nil {
		return nil, err
	}
	// to skip '('
	pulledIncludeWaitingDuration, err := time.ParseDuration(splits[6][1:])
	if err != nil {
		return nil, err
	}
	return &pulledStruct{
		pulledDuration:               pulledDuration,
		pulledIncludeWaitingDuration: pulledIncludeWaitingDuration,
	}, nil
}

/*
Copyright 2016 The Kubernetes Authors.

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

package images

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	crierrors "k8s.io/cri-api/pkg/errors"
	"k8s.io/kubernetes/pkg/credentialprovider"
	credentialprovidersecrets "k8s.io/kubernetes/pkg/credentialprovider/secrets"
	"k8s.io/kubernetes/pkg/features"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/pkg/util/parsers"
)

const (
	// secretPulledImageFileName is the file name where image manager stores state for ensured secret pulled image digeset
	secretPulledImageFileName = "image_manager_state"
	ensuredExpirationDuration = 1 * time.Hour
)

type EnsuredBySecret struct {
	// true for ensured secret
	Ensured bool `json:"ensured"`
	// the secret should be verified again if the dueDate is before now()
	// and the due date is 1 hour after ensured. See const `ensuredExpirationDuration`.
	DueDate time.Time `json:"dueDate"`
}

type ensuredSecretPulledImageDigest struct {
	// map of auth hash (keys) used to successfully pull this imageref;
	// there is a due date in EnsuredBySecret which is 1 hour later after ensured
	HashMatch map[string]*EnsuredBySecret `json:"authHash"`

	ImageName string `json:"name"`
}

type ImagePodPullingTimeRecorder interface {
	RecordImageStartedPulling(podUID types.UID)
	RecordImageFinishedPulling(podUID types.UID)
}

// imageManager provides the functionalities for image pulling.
type imageManager struct {
	recorder     record.EventRecorder
	imageService kubecontainer.ImageService
	backOff      *flowcontrol.Backoff
	// It will check the presence of the image, and report the 'image pulling', image pulled' events correspondingly.
	puller imagePuller

	podPullingTimeRecorder ImagePodPullingTimeRecorder

	keyring *credentialprovider.DockerKeyring

	state ImageState
}

var _ ImageManager = &imageManager{}

// NewImageManager instantiates a new ImageManager object.
func NewImageManager(recorder record.EventRecorder, imageService kubecontainer.ImageService, imageBackOff *flowcontrol.Backoff, serialized bool, maxParallelImagePulls *int32, qps float32, burst int, podPullingTimeRecorder ImagePodPullingTimeRecorder, keyring *credentialprovider.DockerKeyring, stateFileDirectory string) ImageManager {
	imageService = throttleImagePulling(imageService, qps, burst)

	var puller imagePuller
	if serialized {
		puller = newSerialImagePuller(imageService)
	} else {
		puller = newParallelImagePuller(imageService, maxParallelImagePulls)
	}

	stateImpl, err := NewCheckpointState(stateFileDirectory, secretPulledImageFileName)
	// TODO (pacoxu) make a Noop state implementation for test
	if err != nil {
		klog.ErrorS(err, "Could not initialize checkpoint manager.")
	}

	return &imageManager{
		recorder:               recorder,
		imageService:           imageService,
		backOff:                imageBackOff,
		puller:                 puller,
		podPullingTimeRecorder: podPullingTimeRecorder,
		keyring:                keyring,
		state:                  stateImpl,
	}
}

// shouldPullImage returns whether we should pull an image according to
// the presence and pull policy of the image.
func shouldPullImage(container *v1.Container, imagePresent, pulledBySecret, ensuredBySecret bool) bool {
	if container.ImagePullPolicy == v1.PullNever {
		return false
	}

	if !utilfeature.DefaultFeatureGate.Enabled(features.KubeletEnsureSecretPulledImages) { // ungated old behavior
		if container.ImagePullPolicy == v1.PullAlways ||
			(container.ImagePullPolicy == v1.PullIfNotPresent && (!imagePresent)) {
			return true
		}
	} else { // new gated behavior
		if container.ImagePullPolicy == v1.PullAlways {
			return true
		}
		if container.ImagePullPolicy == v1.PullIfNotPresent {
			if !imagePresent {
				return true
			}
			// if the imageRef has been pulled by a secret and Pull Policy is PullIfNotPresent
			// we need to ensure that the current pod's secrets map to an auth that has Already
			// pulled the image successfully. Otherwise pod B could use pod A's images
			// without auth. So in this case if pulledBySecret but not ensured by matching
			// secret auth for a pull again for the pod B scenario where the auth does not match
			if pulledBySecret && !ensuredBySecret {
				return true // noting here that old behaviour returns false in this case indicating the image should not be pulled
			}
		}
	}
	return false
}

func (e *ensuredSecretPulledImageDigest) Clone() *ensuredSecretPulledImageDigest {
	c := &ensuredSecretPulledImageDigest{
		HashMatch: make(map[string]*EnsuredBySecret),
	}
	for k, v := range e.HashMatch {
		c.HashMatch[k] = v
	}
	c.ImageName = e.ImageName
	return c
}

func (m *imageManager) StartGC() {
	klog.InfoS("Starting Kubelet image pull secret ensuring GC manager")
	go wait.Until(func() { m.state.ClearExpiredState() }, 1*time.Hour, wait.NeverStop)
}

// records an event using ref, event msg.  log to glog using prefix, msg, logFn
func (m *imageManager) logIt(ref *v1.ObjectReference, eventtype, event, prefix, msg string, logFn func(args ...interface{})) {
	if ref != nil {
		m.recorder.Event(ref, eventtype, event, msg)
	} else {
		logFn(fmt.Sprint(prefix, " ", msg))
	}
}

// EnsureImageExists pulls the image for the specified pod and container, and returns
// (imageRef, error message, error).
func (m *imageManager) EnsureImageExists(ctx context.Context, pod *v1.Pod, container *v1.Container, pullSecrets []v1.Secret, podSandboxConfig *runtimeapi.PodSandboxConfig, podRuntimeHandler string) (string, string, error) {
	logPrefix := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Image)
	ref, err := kubecontainer.GenerateContainerRef(pod, container)
	if err != nil {
		klog.ErrorS(err, "Couldn't make a ref to pod", "pod", klog.KObj(pod), "containerName", container.Name)
	}

	// If the image contains no tag or digest, a default tag should be applied.
	image, err := applyDefaultImageTag(container.Image)
	if err != nil {
		msg := fmt.Sprintf("Failed to apply default image tag %q: %v", container.Image, err)
		m.logIt(ref, v1.EventTypeWarning, events.FailedToInspectImage, logPrefix, msg, klog.Warning)
		return "", msg, ErrInvalidImageName
	}

	var podAnnotations []kubecontainer.Annotation
	for k, v := range pod.GetAnnotations() {
		podAnnotations = append(podAnnotations, kubecontainer.Annotation{
			Name:  k,
			Value: v,
		})
	}

	spec := kubecontainer.ImageSpec{
		Image:          image,
		Annotations:    podAnnotations,
		RuntimeHandler: podRuntimeHandler,
	}

	imageRef, err := m.imageService.GetImageRef(ctx, spec)
	if err != nil {
		msg := fmt.Sprintf("Failed to inspect image %q: %v", container.Image, err)
		m.logIt(ref, v1.EventTypeWarning, events.FailedToInspectImage, logPrefix, msg, klog.Warning)
		return "", msg, ErrImageInspect
	}

	present := imageRef != ""

	var pulledBySecret, ensuredBySecret bool
	if present {
		pulledBySecret, ensuredBySecret = m.isEnsuredBySecret(imageRef, spec, pullSecrets)
		klog.V(5).InfoS("Get ensured check by securet", "image", image, "imageRef", imageRef, "pulledBySecret", pulledBySecret, "ensuredBySecret", ensuredBySecret)
	}

	if !shouldPullImage(container, present, pulledBySecret, ensuredBySecret) {
		// should not pull when pull never, or if present and correctly authenticated
		if present {
			if utilfeature.DefaultFeatureGate.Enabled(features.KubeletEnsureSecretPulledImages) && (pulledBySecret && !ensuredBySecret) {
				// TODO: add integration test for this thrown error message
				msg := fmt.Sprintf("Container image %q is present with pull policy of Never but does not have the proper auth (image secret) that was used to pull the image", container.Image)
				m.logIt(ref, v1.EventTypeWarning, events.ErrImageNeverPullPolicy, logPrefix, msg, klog.Warning)
				return "", msg, ErrImageNeverPull
			}
			msg := fmt.Sprintf("Container image %q already present on machine", container.Image)
			m.logIt(ref, v1.EventTypeNormal, events.PulledImage, logPrefix, msg, klog.Info)
			return imageRef, "", nil
		}
		msg := fmt.Sprintf("Container image %q is not present with pull policy of Never", container.Image)
		m.logIt(ref, v1.EventTypeWarning, events.ErrImageNeverPullPolicy, logPrefix, msg, klog.Warning)
		return "", msg, ErrImageNeverPull
	}

	backOffKey := fmt.Sprintf("%s_%s", pod.UID, container.Image)
	if m.backOff.IsInBackOffSinceUpdate(backOffKey, m.backOff.Clock.Now()) {
		msg := fmt.Sprintf("Back-off pulling image %q", container.Image)
		m.logIt(ref, v1.EventTypeNormal, events.BackOffPullImage, logPrefix, msg, klog.Info)
		return "", msg, ErrImagePullBackOff
	}
	m.podPullingTimeRecorder.RecordImageStartedPulling(pod.UID)
	m.logIt(ref, v1.EventTypeNormal, events.PullingImage, logPrefix, fmt.Sprintf("Pulling image %q", container.Image), klog.Info)
	startTime := time.Now()
	pullChan := make(chan pullResult)
	m.puller.pullImage(ctx, spec, pullSecrets, pullChan, podSandboxConfig)
	imagePullResult := <-pullChan
	if imagePullResult.err != nil {
		m.logIt(ref, v1.EventTypeWarning, events.FailedToPullImage, logPrefix, fmt.Sprintf("Failed to pull image %q: %v", container.Image, imagePullResult.err), klog.Warning)
		m.backOff.Next(backOffKey, m.backOff.Clock.Now())

		msg, err := evalCRIPullErr(container, imagePullResult.err)
		return "", msg, err
	}
	m.podPullingTimeRecorder.RecordImageFinishedPulling(pod.UID)
	m.logIt(ref, v1.EventTypeNormal, events.PulledImage, logPrefix, fmt.Sprintf("Successfully pulled image %q in %v (%v including waiting)",
		container.Image, imagePullResult.pullDuration.Truncate(time.Millisecond), time.Since(startTime).Truncate(time.Millisecond)), klog.Info)
	m.backOff.GC()

	if m.state != nil {
		if imagePullResult.pullCredentialsHash == "" {
			// successful pull no auth hash returned, auth was not required so we should reset the hashmap for this
			// imageref since auth is no longer required for the local image cache, allowing use of the ImageRef
			// by other pods if it remains cached and pull policy is PullIfNotPresent
			klog.V(4).InfoS("Delete ensured image", "image", image, "imageRef", imagePullResult.imageRef)
			m.state.DeleteEnsuredImage(imagePullResult.imageRef)
		} else {
			// store/create hashMatch map entry for auth config hash key used to pull the image
			// for this imageref (digest)
			klog.V(4).InfoS("Set ensured image", "image", image, "imageRef", imagePullResult.imageRef, "authHash", imagePullResult.pullCredentialsHash)
			m.state.SetEnsured(imagePullResult.imageRef, image, imagePullResult.pullCredentialsHash, true)
		}
	}

	return imagePullResult.imageRef, "", nil
}

func evalCRIPullErr(container *v1.Container, err error) (errMsg string, errRes error) {
	// Error assertions via errors.Is is not supported by gRPC (remote runtime) errors right now.
	// See https://github.com/grpc/grpc-go/issues/3616
	if strings.HasPrefix(err.Error(), crierrors.ErrRegistryUnavailable.Error()) {
		errMsg = fmt.Sprintf(
			"image pull failed for %s because the registry is unavailable%s",
			container.Image,
			// Trim the error name from the message to convert errors like:
			// "RegistryUnavailable: a more detailed explanation" to:
			// "...because the registry is unavailable: a more detailed explanation"
			strings.TrimPrefix(err.Error(), crierrors.ErrRegistryUnavailable.Error()),
		)
		return errMsg, crierrors.ErrRegistryUnavailable
	}

	if strings.HasPrefix(err.Error(), crierrors.ErrSignatureValidationFailed.Error()) {
		errMsg = fmt.Sprintf(
			"image pull failed for %s because the signature validation failed%s",
			container.Image,
			// Trim the error name from the message to convert errors like:
			// "SignatureValidationFailed: a more detailed explanation" to:
			// "...because the signature validation failed: a more detailed explanation"
			strings.TrimPrefix(err.Error(), crierrors.ErrSignatureValidationFailed.Error()),
		)
		return errMsg, crierrors.ErrSignatureValidationFailed
	}

	// Fallback for no specific error
	return err.Error(), ErrImagePull
}

// applyDefaultImageTag parses a docker image string, if it doesn't contain any tag or digest,
// a default tag will be applied.
func applyDefaultImageTag(image string) (string, error) {
	_, tag, digest, err := parsers.ParseImageName(image)
	if err != nil {
		return "", err
	}
	// we just concatenate the image name with the default tag here instead
	if len(digest) == 0 && len(tag) > 0 && !strings.HasSuffix(image, ":"+tag) {
		// we just concatenate the image name with the default tag here instead
		// of using dockerref.WithTag(named, ...) because that would cause the
		// image to be fully qualified as docker.io/$name if it's a short name
		// (e.g. just busybox). We don't want that to happen to keep the CRI
		// agnostic wrt image names and default hostnames.
		image = image + ":" + tag
	}
	return image, nil
}

// isEnsuredBySecret returns two boolean values: pulledBySecret, ensuredBySecret.
// 1. pulledBySecret return true is it is already in the ensured map, and it will
// be removed from the ensured map once it is expired.
// 2. ensuredBySecret returns true if the secret for an auth used to pull an
// image has already been authenticated through a successful pull request
// and the same auth exists for this podSandbox/image/.
func (m *imageManager) isEnsuredBySecret(imageRef string, image kubecontainer.ImageSpec, pullSecrets []v1.Secret) (pulledBySecret, ensuredBySecret bool) {
	if imageRef == "" {
		return
	}
	if m.state == nil {
		return
	}
	pulledBySecret = m.state.GetEnsuredImage(imageRef)

	img := image.Image
	repoToPull, _, _, err := parsers.ParseImageName(img)
	if err != nil {
		return
	}

	if m.keyring == nil {
		return
	}
	keyring, err := credentialprovidersecrets.MakeDockerKeyring(pullSecrets, *m.keyring)
	if err != nil {
		return
	}

	creds, withCredentials := keyring.Lookup(repoToPull)
	if !withCredentials {
		return
	}

	for _, currentCreds := range creds {
		auth := &runtimeapi.AuthConfig{
			Username:      currentCreds.Username,
			Password:      currentCreds.Password,
			Auth:          currentCreds.Auth,
			ServerAddress: currentCreds.ServerAddress,
			IdentityToken: currentCreds.IdentityToken,
			RegistryToken: currentCreds.RegistryToken,
		}

		hash := kubecontainer.HashAuth(auth)
		if hash != "" {
			ensuredBySecret = m.state.GetEnsuredImageWithHash(imageRef, hash)
			if ensuredBySecret {
				return
			}
		}
	}
	return
}

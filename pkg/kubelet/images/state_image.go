/*
Copyright 2023 The Kubernetes Authors.

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
	"sync"
	"time"

	"k8s.io/klog/v2"
)

type stateImageAuthHash struct {
	sync.RWMutex
	ImageMap EnsureSecretPulledImageMap
}

var _ ImageState = &stateImageAuthHash{}

// NewImagePullSecretState creates new State for keeping track of ensure secret pulled image state
func NewImageAuthHashState() ImageState {
	klog.InfoS("Initialized new in-memory image state store")
	return &stateImageAuthHash{
		ImageMap: EnsureSecretPulledImageMap{},
	}
}

// get all states
func (s *stateImageAuthHash) GetEnsureSecretPulledImageMap() EnsureSecretPulledImageMap {
	s.RLock()
	defer s.RUnlock()
	return s.ImageMap.Clone()
}

// check the current state
func (s *stateImageAuthHash) GetEnsuredImage(imageRef string) bool {
	s.RLock()
	defer s.RUnlock()
	_, ok := s.ImageMap[imageRef]
	return ok
}

// check the current state
func (s *stateImageAuthHash) GetEnsuredImageWithHash(imageRef string, authHash string) bool {
	s.RLock()
	defer s.RUnlock()
	digest, ok := s.ImageMap[imageRef]
	if !ok {
		return false
	}
	ensuredBySecret, ok := digest.HashMatch[authHash]
	if !ok {
		return false
	}
	if time.Now().Before(ensuredBySecret.DueDate) {
		return ensuredBySecret.Ensured
	}
	return false
}

func (s *stateImageAuthHash) SetEnsureSecretPulledImageMap(ImageMap EnsureSecretPulledImageMap) {
	s.RLock()
	defer s.RUnlock()
	s.ImageMap = ImageMap
}

func (s *stateImageAuthHash) SetEnsured(imageRef, imageName, authHash string, ensured bool) {
	s.Lock()
	defer s.Unlock()
	klog.V(2).InfoS("Set image auth hash state", "imageRef", imageRef, "authHash", authHash, "ensured", ensured)
	digest, ok := s.ImageMap[imageRef]
	if !ok {
		digest = &ensuredSecretPulledImageDigest{
			HashMatch: make(map[string]*EnsuredBySecret),
			ImageName: imageName,
		}
		s.ImageMap[imageRef] = digest
	}
	ensuredBySecret, ok := digest.HashMatch[authHash]
	if !ok {
		digest.HashMatch[authHash] = &EnsuredBySecret{
			Ensured: ensured,
			DueDate: time.Now().Add(time.Hour),
		}
	} else {
		ensuredBySecret.Ensured = ensured
		ensuredBySecret.DueDate = time.Now().Add(time.Hour)
	}
}

func (s *stateImageAuthHash) DeleteEnsuredImage(imageRef string) {
	s.Lock()
	defer s.Unlock()
	delete(s.ImageMap, imageRef)
	klog.V(4).InfoS("Deleted image auth hash state", "imageRef", imageRef)
}

func (s *stateImageAuthHash) DeleteEnsuredImageAuthHash(imageRef string, authHash string) {
	s.Lock()
	defer s.Unlock()
	digest, ok := s.ImageMap[imageRef]
	if ok {
		delete(digest.HashMatch, authHash)
		klog.V(4).InfoS("Deleted image auth hash state", "imageRef", imageRef, "authHash", authHash)
	}
}

func (s *stateImageAuthHash) ClearExpiredState() {
	s.Lock()
	defer s.Unlock()
	// TODO (pacoxu) clear dueDate after Now.
	// or do the clean up when storing a new state
	klog.V(4).InfoS("Cleared expired state")
}

func (s *stateImageAuthHash) ClearState() {
	s.Lock()
	defer s.Unlock()
	klog.V(4).InfoS("Cleared state")
}

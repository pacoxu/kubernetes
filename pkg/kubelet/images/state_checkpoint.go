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
	"fmt"
	"path/filepath"
	"sync"

	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager/errors"
)

type stateCheckpoint struct {
	mux               sync.RWMutex
	cache             ImageState
	checkpointManager checkpointmanager.CheckpointManager
	checkpointName    string
}

var _ ImageState = &stateCheckpoint{}

// NewCheckpoint creates new State for keeping track of current ensure secret pulled image state with checkpoint backend
func NewCheckpointState(stateDir, checkpointName string) (ImageState, error) {
	checkpointManager, err := checkpointmanager.NewCheckpointManager(stateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize checkpoint manager: %v", err)
	}
	checkpoint := &stateCheckpoint{
		cache:             NewImageAuthHashState(),
		checkpointManager: checkpointManager,
		checkpointName:    checkpointName,
	}

	if err := checkpoint.restoreState(); err != nil {
		// TODO (pacoxu): clear the file if the restore failed.
		//nolint:staticcheck // ST1005 user-facing error message
		return nil, fmt.Errorf("could not restore state from checkpoint file %q: %v",
			filepath.Join(stateDir, checkpointName), err)
	}

	return checkpoint, nil
}

// restores state from a checkpoint and creates it if it doesn't exist
func (sc *stateCheckpoint) restoreState() error {
	sc.mux.Lock()
	defer sc.mux.Unlock()

	var err error
	checkpointV1 := newImageManagerCheckpointV1()

	if err = sc.checkpointManager.GetCheckpoint(sc.checkpointName, checkpointV1); err != nil {
		if err == errors.ErrCheckpointNotFound {
			return sc.storeState()
		}
		return err
	}

	var tmpImageMap EnsureSecretPulledImageMap
	if checkpointV1.ImageMap != nil {
		tmpImageMap = checkpointV1.ImageMap
	}
	sc.cache.SetEnsureSecretPulledImageMap(tmpImageMap.Clone())

	klog.V(2).InfoS("State checkpoint: restored image state from checkpoint")

	return nil
}

// saves state to a checkpoint, caller is responsible for locking
func (sc *stateCheckpoint) storeState() error {
	checkpoint := NewImageManagerCheckpoint()

	ImageMap := sc.cache.GetEnsureSecretPulledImageMap()
	checkpoint.ImageMap = ImageMap.Clone()

	err := sc.checkpointManager.CreateCheckpoint(sc.checkpointName, checkpoint)
	if err != nil {
		klog.ErrorS(err, "Failed to save checkpoint")
		return err
	}
	return nil
}

// GetEnsureSecretPulledImageMap returns current all image ensured state
func (sc *stateCheckpoint) GetEnsureSecretPulledImageMap() EnsureSecretPulledImageMap {
	sc.mux.RLock()
	defer sc.mux.RUnlock()

	return sc.cache.GetEnsureSecretPulledImageMap()
}

func (sc *stateCheckpoint) SetEnsureSecretPulledImageMap(ImageMap EnsureSecretPulledImageMap) {
	sc.mux.RLock()
	defer sc.mux.RUnlock()

	sc.cache.SetEnsureSecretPulledImageMap(ImageMap)
}

func (sc *stateCheckpoint) GetEnsuredImage(imageRef string) bool {
	sc.mux.RLock()
	defer sc.mux.RUnlock()

	return sc.cache.GetEnsuredImage(imageRef)
}

func (sc *stateCheckpoint) GetEnsuredImageWithHash(imageRef string, authHash string) bool {
	sc.mux.RLock()
	defer sc.mux.RUnlock()

	return sc.cache.GetEnsuredImageWithHash(imageRef, authHash)
}

func (sc *stateCheckpoint) SetEnsured(imageRef, imageName, authHash string, ensured bool) {
	sc.mux.Lock()
	defer sc.mux.Unlock()
	sc.cache.SetEnsured(imageRef, imageName, authHash, ensured)
	err := sc.storeState()
	if err != nil {
		klog.ErrorS(err, "Store state to checkpoint error")
	}
}

func (sc *stateCheckpoint) DeleteEnsuredImage(imageRef string) {
	sc.mux.Lock()
	defer sc.mux.Unlock()

	sc.cache.DeleteEnsuredImage(imageRef)
	err := sc.storeState()
	if err != nil {
		klog.ErrorS(err, "Store state to checkpoint error")
	}
}

func (sc *stateCheckpoint) DeleteEnsuredImageAuthHash(imageRef string, authHash string) {
	sc.mux.Lock()
	defer sc.mux.Unlock()

	sc.cache.DeleteEnsuredImageAuthHash(imageRef, authHash)
	err := sc.storeState()
	if err != nil {
		klog.ErrorS(err, "Store state to checkpoint error")
	}
}

// ClearExpiredState clears expired state and saves it in a checkpoint
func (sc *stateCheckpoint) ClearExpiredState() {
	sc.mux.Lock()
	defer sc.mux.Unlock()
	sc.cache.ClearExpiredState()
	err := sc.storeState()
	if err != nil {
		klog.ErrorS(err, "Store state to checkpoint error")
	}
}

// ClearExpiredState clears state and saves it in a checkpoint
func (sc *stateCheckpoint) ClearState() {
	sc.mux.Lock()
	defer sc.mux.Unlock()
	sc.cache.ClearState()
	err := sc.storeState()
	if err != nil {
		klog.ErrorS(err, "Store state to checkpoint error")
	}
}

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
	"encoding/json"

	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager/checksum"
)

var _ checkpointmanager.Checkpoint = &ImageManagerCheckpointV1{}
var _ checkpointmanager.Checkpoint = &ImageManagerCheckpoint{}

// ImageManagerCheckpoint struct is used to store mage ensured secret state in a checkpoint in v1 format
type ImageManagerCheckpoint struct {
	ImageMap map[string]*ensuredSecretPulledImageDigest `json:"images"`
	Checksum checksum.Checksum
}

// ImageManagerCheckpointV1 struct is used to store mage ensured secret state in a checkpoint in v1 format
type ImageManagerCheckpointV1 = ImageManagerCheckpoint

// NewImageManagerCheckpoint returns an instance of Checkpoint
func NewImageManagerCheckpoint() *ImageManagerCheckpoint {
	return newImageManagerCheckpointV1()
}

func newImageManagerCheckpointV1() *ImageManagerCheckpointV1 {
	return &ImageManagerCheckpointV1{
		ImageMap: make(map[string]*ensuredSecretPulledImageDigest),
	}
}

// MarshalCheckpoint returns marshalled checkpoint in v1 format
func (cp *ImageManagerCheckpointV1) MarshalCheckpoint() ([]byte, error) {
	cp.Checksum = checksum.New(cp.ImageMap)
	return json.Marshal(*cp)
}

// UnmarshalCheckpoint tries to unmarshal passed bytes to checkpoint in v1 format
func (cp *ImageManagerCheckpointV1) UnmarshalCheckpoint(blob []byte) error {
	return json.Unmarshal(blob, cp)
}

// VerifyChecksum is place holder to do verify
func (cp *ImageManagerCheckpointV1) VerifyChecksum() error {
	return cp.Checksum.Verify(cp.ImageMap)
}

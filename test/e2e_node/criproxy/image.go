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

package criproxy

import (
	"context"

	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type imageServiceProxy struct {
	errorInjector func(string) error
	internalapi.ImageManagerService
}

func NewImageServiceProxy(client internalapi.ImageManagerService) internalapi.ImageManagerService {
	return &imageServiceProxy{
		ImageManagerService: client,
	}
}

func (r *imageServiceProxy) SetErrorInjectors(errorInjectors func(string) error) {
	r.errorInjector = errorInjectors
}

func (r *imageServiceProxy) ListImages(ctx context.Context, filter *runtimeapi.ImageFilter) ([]*runtimeapi.Image, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ListImages"); err != nil {
			return nil, err
		}
	}
	return r.ImageManagerService.ListImages(ctx, filter)
}

func (r *imageServiceProxy) ImageStatus(ctx context.Context, image *runtimeapi.ImageSpec, verbose bool) (*runtimeapi.ImageStatusResponse, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ImageStatus"); err != nil {
			return nil, err
		}
	}
	return r.ImageManagerService.ImageStatus(ctx, image, verbose)
}

func (r *imageServiceProxy) PullImage(ctx context.Context, image *runtimeapi.ImageSpec, auth *runtimeapi.AuthConfig, podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("PullImage"); err != nil {
			return "", err
		}
	}
	return r.ImageManagerService.PullImage(ctx, image, auth, podSandboxConfig)
}

func (r *imageServiceProxy) RemoveImage(ctx context.Context, image *runtimeapi.ImageSpec) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("RemoveImage"); err != nil {
			return err
		}
	}
	return r.ImageManagerService.RemoveImage(ctx, image)
}

func (r *imageServiceProxy) ImageFsInfo(ctx context.Context) (*runtimeapi.ImageFsInfoResponse, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ImageFsInfo"); err != nil {
			return nil, err
		}
	}
	return r.ImageManagerService.ImageFsInfo(ctx)
}

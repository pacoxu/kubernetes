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
	"time"

	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type runtimeServiceProxy struct {
	errorInjector func(string) error
	internalapi.RuntimeService
}

func NewRuntimeServiceProxy(client internalapi.RuntimeService) internalapi.RuntimeService {
	return &runtimeServiceProxy{
		RuntimeService: client,
	}
}

func (r *runtimeServiceProxy) SetErrorInjectors(errorInjectors func(string) error) {
	r.errorInjector = errorInjectors
}

func (r *runtimeServiceProxy) Version(ctx context.Context, apiVersion string) (*runtimeapi.VersionResponse, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("Version"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.Version(ctx, apiVersion)
}

func (r *runtimeServiceProxy) RunPodSandbox(ctx context.Context, config *runtimeapi.PodSandboxConfig, runtimeHandler string) (string, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("RunPodSandbox"); err != nil {
			return "", err
		}
	}
	return r.RuntimeService.RunPodSandbox(ctx, config, runtimeHandler)
}

func (r *runtimeServiceProxy) StopPodSandbox(ctx context.Context, podSandBoxID string) (err error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("StopPodSandbox"); err != nil {
			return err
		}
	}
	return r.RuntimeService.StopPodSandbox(ctx, podSandBoxID)
}

func (r *runtimeServiceProxy) RemovePodSandbox(ctx context.Context, podSandboxID string) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("RemovePodSandbox"); err != nil {
			return err
		}
	}
	return r.RuntimeService.RemovePodSandbox(ctx, podSandboxID)
}

func (r *runtimeServiceProxy) PodSandboxStatus(ctx context.Context, podSandboxID string, verbose bool) (*runtimeapi.PodSandboxStatusResponse, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("PodSandboxStatus"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.PodSandboxStatus(ctx, podSandboxID, verbose)
}

func (r *runtimeServiceProxy) ListPodSandbox(ctx context.Context, filter *runtimeapi.PodSandboxFilter) ([]*runtimeapi.PodSandbox, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ListPodSandbox"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.ListPodSandbox(ctx, filter)
}

func (r *runtimeServiceProxy) PortForward(ctx context.Context, request *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("PortForward"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.PortForward(ctx, request)
}

func (r *runtimeServiceProxy) CreateContainer(ctx context.Context, podSandboxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("CreateContainer"); err != nil {
			return "", err
		}
	}
	return r.RuntimeService.CreateContainer(ctx, podSandboxID, config, sandboxConfig)
}

func (r *runtimeServiceProxy) StartContainer(ctx context.Context, containerID string) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("StartContainer"); err != nil {
			return err
		}
	}
	return r.RuntimeService.StartContainer(ctx, containerID)
}

func (r *runtimeServiceProxy) StopContainer(ctx context.Context, containerID string, timeout int64) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("StopContainer"); err != nil {
			return err
		}
	}
	return r.RuntimeService.StopContainer(ctx, containerID, timeout)
}

func (r *runtimeServiceProxy) RemoveContainer(ctx context.Context, containerID string) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("RemoveContainer"); err != nil {
			return err
		}
	}
	return r.RuntimeService.RemoveContainer(ctx, containerID)
}

func (r *runtimeServiceProxy) ListContainers(ctx context.Context, filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ListContainers"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.ListContainers(ctx, filter)
}

func (r *runtimeServiceProxy) ContainerStatus(ctx context.Context, containerID string, verbose bool) (*runtimeapi.ContainerStatusResponse, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ContainerStatus"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.ContainerStatus(ctx, containerID, verbose)
}

func (r *runtimeServiceProxy) UpdateContainerResources(ctx context.Context, containerID string, resources *runtimeapi.ContainerResources) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("UpdateContainerResources"); err != nil {
			return err
		}
	}
	return r.RuntimeService.UpdateContainerResources(ctx, containerID, resources)
}

func (r *runtimeServiceProxy) ExecSync(ctx context.Context, containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ExecSync"); err != nil {
			return nil, nil, err
		}
	}
	return r.RuntimeService.ExecSync(ctx, containerID, cmd, timeout)
}

func (r *runtimeServiceProxy) Exec(ctx context.Context, request *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("Exec"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.Exec(ctx, request)
}

func (r *runtimeServiceProxy) Attach(ctx context.Context, req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("Attach"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.Attach(ctx, req)
}

func (r *runtimeServiceProxy) ReopenContainerLog(ctx context.Context, containerID string) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("ReopenContainerLog"); err != nil {
			return err
		}
	}
	return r.RuntimeService.ReopenContainerLog(ctx, containerID)
}

func (r *runtimeServiceProxy) CheckpointContainer(ctx context.Context, options *runtimeapi.CheckpointContainerRequest) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("CheckpointContainer"); err != nil {
			return err
		}
	}
	return r.RuntimeService.CheckpointContainer(ctx, options)
}

func (r *runtimeServiceProxy) GetContainerEvents(ctx context.Context, containerEventsCh chan *runtimeapi.ContainerEventResponse, connectionEstablishedCallback func(runtimeapi.RuntimeService_GetContainerEventsClient)) error {
	if r.errorInjector != nil {
		if err := r.errorInjector("GetContainerEvents"); err != nil {
			return err
		}
	}
	return r.RuntimeService.GetContainerEvents(ctx, containerEventsCh, connectionEstablishedCallback)
}

func (r *runtimeServiceProxy) ContainerStats(ctx context.Context, containerID string) (*runtimeapi.ContainerStats, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ContainerStats"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.ContainerStats(ctx, containerID)
}
func (r *runtimeServiceProxy) ListContainerStats(ctx context.Context, filter *runtimeapi.ContainerStatsFilter) ([]*runtimeapi.ContainerStats, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ListContainerStats"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.ListContainerStats(ctx, filter)
}

func (r *runtimeServiceProxy) PodSandboxStats(ctx context.Context, podSandboxID string) (*runtimeapi.PodSandboxStats, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("PodSandboxStats"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.PodSandboxStats(ctx, podSandboxID)
}

func (r *runtimeServiceProxy) ListPodSandboxStats(ctx context.Context, filter *runtimeapi.PodSandboxStatsFilter) ([]*runtimeapi.PodSandboxStats, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ListPodSandboxStats"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.ListPodSandboxStats(ctx, filter)
}

func (r *runtimeServiceProxy) ListMetricDescriptors(ctx context.Context) ([]*runtimeapi.MetricDescriptor, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ListMetricDescriptors"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.ListMetricDescriptors(ctx)
}

func (r *runtimeServiceProxy) ListPodSandboxMetrics(ctx context.Context) ([]*runtimeapi.PodSandboxMetrics, error) {
	if r.errorInjector != nil {
		if err := r.errorInjector("ListPodSandboxMetrics"); err != nil {
			return nil, err
		}
	}
	return r.RuntimeService.ListPodSandboxMetrics(ctx)
}

/*
Copyright 2022 The Kubernetes Authors.

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

package cm

import (
	"fmt"
	"reflect"
	"sync"
)

type FakeCgroupManager struct {
	sync.Mutex
	CalledFunctions []string
}

func NewFakeCgroupManager() *FakeCgroupManager {
	return &FakeCgroupManager{}
}

func (cm *FakeCgroupManager) Create(*CgroupConfig) error {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "Create")
	return nil
}

func (cm *FakeCgroupManager) Destroy(*CgroupConfig) error {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "Destroy")
	return nil
}

func (cm *FakeCgroupManager) Update(*CgroupConfig) error {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "Update")
	return nil
}

func (cm *FakeCgroupManager) Validate(name CgroupName) error {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "Validate")
	return nil
}

func (cm *FakeCgroupManager) Exists(name CgroupName) bool {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "Exists")
	return false
}

func (cm *FakeCgroupManager) Name(name CgroupName) string {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "Name")
	return name.ToCgroupfs()
}

func (cm *FakeCgroupManager) CgroupName(name string) CgroupName {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "CgroupName")
	return ParseCgroupfsToCgroupName(name)
}

func (cm *FakeCgroupManager) Pids(name CgroupName) []int {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "Pids")
	return []int{}
}

func (cm *FakeCgroupManager) ReduceCPULimits(cgroupName CgroupName) error {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "ReduceCPULimits")
	return nil
}

func (cm *FakeCgroupManager) MemoryUsage(name CgroupName) (int64, error) {
	cm.Lock()
	defer cm.Unlock()
	cm.CalledFunctions = append(cm.CalledFunctions, "MemoryUsage")
	return 0, nil
}

func (cm *FakeCgroupManager) AssertCalls(calls []string) error {
	cm.Lock()
	defer cm.Unlock()

	if !reflect.DeepEqual(calls, cm.CalledFunctions) {
		return fmt.Errorf("expected %#v, got %#v", calls, cm.CalledFunctions)
	}
	return nil
}

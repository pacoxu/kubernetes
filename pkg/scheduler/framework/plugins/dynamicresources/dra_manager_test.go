/*
Copyright The Kubernetes Authors.

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

package dynamicresources

import (
	"fmt"
	"maps"
	"sync"
	"testing"

	"github.com/onsi/gomega"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	"k8s.io/kubernetes/pkg/scheduler/util/assumecache"
	"k8s.io/kubernetes/test/utils/ktesting"
)

const (
	claimUID      = types.UID("claim-uid-1")
	otherClaimUID = types.UID("claim-uid-2")
)

// testInformer implements [assumecache.Informer] and can be used to feed changes into an assume
// cache during unit testing. Only a single event handler is supported, which is
// sufficient for one assume cache.
type testInformer struct {
	handler cache.ResourceEventHandler
}

func (i *testInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	i.handler = handler
	return nil, nil
}

func (i *testInformer) add(obj any) {
	if i.handler == nil {
		return
	}
	i.handler.OnAdd(obj, false)
}

func TestSignalClaimPendingAllocation(t *testing.T) {
	testSignalClaimPendingAllocation(ktesting.Init(t))
}
func testSignalClaimPendingAllocation(tCtx ktesting.TContext) {
	specialAllocatedClaim := allocatedClaim.DeepCopy()
	specialAllocatedClaim.Name = specialClaimInMemName

	tests := map[string]struct {
		inFlightAllocations         map[types.UID]inFlightAllocation
		cachedClaims                []*resourceapi.ResourceClaim
		claimUID                    types.UID
		allocatedClaim              *resourceapi.ResourceClaim
		expectedInFlightAllocations map[types.UID]inFlightAllocation
	}{
		"empty": {
			cachedClaims: []*resourceapi.ResourceClaim{
				st.FromResourceClaim(pendingClaim).UID(string(claimUID)).Obj(),
			},
			claimUID:       claimUID,
			allocatedClaim: allocatedClaim,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID: {claim: allocatedClaim, sharers: 1},
			},
		},
		"claim-already-allocated": {
			claimUID: claimUID,
			cachedClaims: []*resourceapi.ResourceClaim{
				st.FromResourceClaim(allocatedClaim).UID(string(claimUID)).Obj(),
			},
			allocatedClaim:              allocatedClaim,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{},
		},
		"extended-resource-claim-not-in-cache": {
			claimUID:       claimUID,
			allocatedClaim: specialAllocatedClaim,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID: {claim: specialAllocatedClaim, sharers: 1},
			},
		},
		"already-exists": {
			inFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID:      {claim: allocatedClaim, sharers: 1},
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
			claimUID:       claimUID,
			allocatedClaim: allocatedClaim,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID:      {claim: allocatedClaim, sharers: 2},
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
		},
		"already-exists-ignores-different-claim-argument": {
			inFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID: {claim: allocatedClaim, sharers: 1},
			},
			claimUID:       claimUID,
			allocatedClaim: allocatedClaim2,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID: {claim: allocatedClaim, sharers: 2},
			},
		},
	}

	for name, test := range tests {
		tCtx.Run(name, func(tCtx ktesting.TContext) {
			c := &claimTracker{
				logger:              tCtx.Logger(),
				inFlightAllocations: make(map[types.UID]inFlightAllocation),
			}
			maps.Copy(c.inFlightAllocations, test.inFlightAllocations)

			informer := &testInformer{}
			c.cache = assumecache.NewAssumeCache(tCtx.Logger(), informer, "", "", nil)
			for _, cached := range test.cachedClaims {
				informer.add(cached)
			}

			err := c.SignalClaimPendingAllocation(test.claimUID, test.allocatedClaim)
			tCtx.ExpectNoError(err)
			tCtx.Expect(c.inFlightAllocations).To(gomega.Equal(test.expectedInFlightAllocations))
		})
	}
}

func TestGetPendingAllocation(t *testing.T) {
	testGetPendingAllocation(ktesting.Init(t))
}
func testGetPendingAllocation(tCtx ktesting.TContext) {
	tests := map[string]struct {
		inFlightAllocations map[types.UID]inFlightAllocation
		claimUID            types.UID
		expected            *resourceapi.AllocationResult
	}{
		"empty": {
			claimUID: claimUID,
			expected: nil,
		},
		"nil-claim": {
			inFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID: {claim: nil},
			},
			claimUID: claimUID,
			expected: nil,
		},
		"claim": {
			inFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID:      {claim: allocatedClaim, sharers: 1},
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
			claimUID: claimUID,
			expected: allocationResult,
		},
	}

	for name, test := range tests {
		tCtx.Run(name, func(tCtx ktesting.TContext) {
			c := &claimTracker{
				logger:              tCtx.Logger(),
				inFlightAllocations: make(map[types.UID]inFlightAllocation),
			}
			maps.Copy(c.inFlightAllocations, test.inFlightAllocations)
			beforeInFlight := maps.Clone(c.inFlightAllocations)

			actual := c.GetPendingAllocation(test.claimUID)
			tCtx.Expect(actual).To(gomega.Equal(test.expected))
			// Get is strictly read-only
			tCtx.Expect(c.inFlightAllocations).To(gomega.Equal(beforeInFlight))
		})
	}
}

func TestMaybeRemoveClaimPendingAllocation(t *testing.T) {
	testMaybeRemoveClaimPendingAllocation(ktesting.Init(t))
}
func testMaybeRemoveClaimPendingAllocation(tCtx ktesting.TContext) {
	tests := map[string]struct {
		inFlightAllocations         map[types.UID]inFlightAllocation
		claimUID                    types.UID
		forceRemove                 bool
		expected                    bool
		expectedInFlightAllocations map[types.UID]inFlightAllocation
	}{
		"empty": {
			claimUID:                    claimUID,
			forceRemove:                 true,
			expected:                    false,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{},
		},
		"delete-last-sharer": {
			inFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID:      {claim: allocatedClaim, sharers: 1},
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
			claimUID: claimUID,
			expected: true,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
		},
		"force-delete-last-sharer": {
			inFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID:      {claim: allocatedClaim, sharers: 1},
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
			claimUID:    claimUID,
			forceRemove: true,
			expected:    true,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
		},
		"decrement-remaining-sharers": {
			inFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID:      {claim: allocatedClaim, sharers: 2},
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
			claimUID: claimUID,
			expected: false,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID:      {claim: allocatedClaim, sharers: 1},
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
		},
		"force-delete-remaining-sharers": {
			inFlightAllocations: map[types.UID]inFlightAllocation{
				claimUID:      {claim: allocatedClaim, sharers: 2},
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
			claimUID:    claimUID,
			forceRemove: true,
			expected:    true,
			expectedInFlightAllocations: map[types.UID]inFlightAllocation{
				otherClaimUID: {claim: allocatedClaim2, sharers: 10},
			},
		},
	}

	for name, test := range tests {
		tCtx.Run(name, func(tCtx ktesting.TContext) {
			c := &claimTracker{
				logger:              tCtx.Logger(),
				inFlightAllocations: make(map[types.UID]inFlightAllocation),
			}
			maps.Copy(c.inFlightAllocations, test.inFlightAllocations)

			actual := c.MaybeRemoveClaimPendingAllocation(test.claimUID, test.forceRemove)
			tCtx.Expect(actual).To(gomega.Equal(test.expected), "wrong value for deletion indicator")
			tCtx.Expect(c.inFlightAllocations).To(gomega.Equal(test.expectedInFlightAllocations))
		})
	}
}

func TestScopedPendingAllocationLifecycle(t *testing.T) {
	tCtx := ktesting.Init(t)
	tracker := newScopedClaimTracker(tCtx)
	podA := types.UID("pod-a")
	podB := types.UID("pod-b")
	podGroup := types.UID("pod-group")

	handleA, err := tracker.signalScopedClaimPendingAllocation(claimUID, allocatedClaim, podGroup, podA, true)
	tCtx.ExpectNoError(err)

	// Inspect must not acquire a reference. Reserve performs the acquisition.
	handleB, err := tracker.inspectPendingAllocation(claimUID, podGroup, podB)
	tCtx.ExpectNoError(err)
	tCtx.Expect(handleB).NotTo(gomega.BeNil())
	tCtx.Expect(tracker.inFlightAllocations[claimUID].podSharers).To(gomega.And(gomega.HaveLen(1), gomega.HaveKey(podA)))

	allocation, err := tracker.acquirePendingAllocation(handleB, podGroup, podB)
	tCtx.ExpectNoError(err)
	tCtx.Expect(allocation).To(gomega.Equal(allocationResult))
	tCtx.Expect(tracker.inFlightAllocations[claimUID].podSharers).To(gomega.HaveLen(2))

	_, err = tracker.inspectPendingAllocation(claimUID, types.UID("other-pod-group"), types.UID("pod-c"))
	tCtx.Expect(err).To(gomega.MatchError(errPendingAllocationNotShareable))

	release := tracker.releasePendingAllocation(handleA, podA)
	tCtx.Expect(release).To(gomega.Equal(pendingAllocationRelease{}))
	publishedClaim := allocatedClaim.DeepCopy()
	publishedClaim.Status.ReservedFor = []resourceapi.ResourceClaimConsumerReference{{Resource: "pods", Name: "pod-b", UID: podB}}
	tracker.markPendingAllocationPublished(handleB, publishedClaim)

	release = tracker.releasePendingAllocation(handleB, podB)
	tCtx.Expect(release).To(gomega.Equal(pendingAllocationRelease{deleted: true, published: true}))
	tCtx.Expect(tracker.inFlightAllocations).To(gomega.BeEmpty())
	// Release is idempotent.
	tCtx.Expect(tracker.releasePendingAllocation(handleB, podB)).To(gomega.Equal(pendingAllocationRelease{}))
}

func TestScopedPendingAllocationNotShareable(t *testing.T) {
	tCtx := ktesting.Init(t)
	tracker := newScopedClaimTracker(tCtx)
	podA := types.UID("pod-a")
	podGroup := types.UID("pod-group")

	handle, err := tracker.signalScopedClaimPendingAllocation(claimUID, allocatedClaim, podGroup, podA, false)
	tCtx.ExpectNoError(err)
	_, err = tracker.inspectPendingAllocation(claimUID, podGroup, types.UID("pod-b"))
	tCtx.Expect(err).To(gomega.MatchError(errPendingAllocationNotShareable))

	// The creating Pod may acquire the same handle again without adding a
	// duplicate reference. This makes Reserve retries idempotent.
	allocation, err := tracker.acquirePendingAllocation(handle, podGroup, podA)
	tCtx.ExpectNoError(err)
	tCtx.Expect(allocation).To(gomega.Equal(allocationResult))
	tCtx.Expect(tracker.inFlightAllocations[claimUID].podSharers).To(gomega.HaveLen(1))
}

func TestScopedPendingAllocationRejectsStaleHandle(t *testing.T) {
	tCtx := ktesting.Init(t)
	tracker := newScopedClaimTracker(tCtx)
	podA := types.UID("pod-a")
	podB := types.UID("pod-b")
	podGroup := types.UID("pod-group")

	handleA, err := tracker.signalScopedClaimPendingAllocation(claimUID, allocatedClaim, podGroup, podA, true)
	tCtx.ExpectNoError(err)
	staleHandle, err := tracker.inspectPendingAllocation(claimUID, podGroup, podB)
	tCtx.ExpectNoError(err)
	tCtx.Expect(tracker.releasePendingAllocation(handleA, podA).deleted).To(gomega.BeTrue())

	_, err = tracker.signalScopedClaimPendingAllocation(claimUID, allocatedClaim, podGroup, podA, true)
	tCtx.ExpectNoError(err)
	_, err = tracker.acquirePendingAllocation(staleHandle, podGroup, podB)
	tCtx.Expect(err).To(gomega.MatchError(errPendingAllocationChanged))
}

func TestScopedPendingAllocationConcurrentAcquire(t *testing.T) {
	tCtx := ktesting.Init(t)
	tracker := newScopedClaimTracker(tCtx)
	podGroup := types.UID("pod-group")
	owner := types.UID("pod-owner")

	_, err := tracker.signalScopedClaimPendingAllocation(claimUID, allocatedClaim, podGroup, owner, true)
	tCtx.ExpectNoError(err)

	const numSharers = 32
	var wg sync.WaitGroup
	errorsCh := make(chan error, numSharers)
	for i := range numSharers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			podUID := types.UID(fmt.Sprintf("pod-%d", i))
			handle, err := tracker.inspectPendingAllocation(claimUID, podGroup, podUID)
			if err == nil {
				_, err = tracker.acquirePendingAllocation(handle, podGroup, podUID)
			}
			if err != nil {
				errorsCh <- err
			}
		}()
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		tCtx.Errorf("acquire pending allocation: %v", err)
	}
	tCtx.Expect(tracker.inFlightAllocations[claimUID].podSharers).To(gomega.HaveLen(numSharers + 1))
}

func newScopedClaimTracker(tCtx ktesting.TContext) *claimTracker {
	informer := &testInformer{}
	tracker := &claimTracker{
		cache:               assumecache.NewAssumeCache(tCtx.Logger(), informer, "", "", nil),
		inFlightAllocations: make(map[types.UID]inFlightAllocation),
		logger:              tCtx.Logger(),
	}
	informer.add(st.FromResourceClaim(pendingClaim).UID(string(claimUID)).Obj())
	return tracker
}

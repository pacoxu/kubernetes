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

package validation

import (
	"fmt"
	"testing"
	"time"

	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/policy"
)

func TestValidatePodDisruptionBudgetSpec(t *testing.T) {
	minAvailable := intstr.FromString("0%")
	maxUnavailable := intstr.FromString("10%")

	spec := policy.PodDisruptionBudgetSpec{
		MinAvailable:   &minAvailable,
		MaxUnavailable: &maxUnavailable,
	}
	errs := ValidatePodDisruptionBudgetSpec(spec, field.NewPath("foo"))
	if len(errs) == 0 {
		t.Errorf("unexpected success for %v", spec)
	}
}

func TestValidateMinAvailablePodDisruptionBudgetSpec(t *testing.T) {
	successCases := []intstr.IntOrString{
		intstr.FromString("0%"),
		intstr.FromString("1%"),
		intstr.FromString("100%"),
		intstr.FromInt(0),
		intstr.FromInt(1),
		intstr.FromInt(100),
	}
	for _, c := range successCases {
		spec := policy.PodDisruptionBudgetSpec{
			MinAvailable: &c,
		}
		errs := ValidatePodDisruptionBudgetSpec(spec, field.NewPath("foo"))
		if len(errs) != 0 {
			t.Errorf("unexpected failure %v for %v", errs, spec)
		}
	}

	failureCases := []intstr.IntOrString{
		intstr.FromString("1.1%"),
		intstr.FromString("nope"),
		intstr.FromString("-1%"),
		intstr.FromString("101%"),
		intstr.FromInt(-1),
	}
	for _, c := range failureCases {
		spec := policy.PodDisruptionBudgetSpec{
			MinAvailable: &c,
		}
		errs := ValidatePodDisruptionBudgetSpec(spec, field.NewPath("foo"))
		if len(errs) == 0 {
			t.Errorf("unexpected success for %v", spec)
		}
	}
}

func TestValidateMinAvailablePodAndMaxUnavailableDisruptionBudgetSpec(t *testing.T) {
	c1 := intstr.FromString("10%")
	c2 := intstr.FromInt(1)

	spec := policy.PodDisruptionBudgetSpec{
		MinAvailable:   &c1,
		MaxUnavailable: &c2,
	}
	errs := ValidatePodDisruptionBudgetSpec(spec, field.NewPath("foo"))
	if len(errs) == 0 {
		t.Errorf("unexpected success for %v", spec)
	}
}

func TestValidatePodDisruptionBudgetStatus(t *testing.T) {
	const expectNoErrors = false
	const expectErrors = true
	testCases := []struct {
		name                string
		pdbStatus           policy.PodDisruptionBudgetStatus
		expectErrForVersion map[schema.GroupVersion]bool
	}{
		{
			name: "DisruptionsAllowed: 10",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				DisruptionsAllowed: 10,
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectNoErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "CurrentHealthy: 5",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				CurrentHealthy: 5,
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectNoErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "DesiredHealthy: 3",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				DesiredHealthy: 3,
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectNoErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "ExpectedPods: 2",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				ExpectedPods: 2,
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectNoErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "DisruptionsAllowed: -10",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				DisruptionsAllowed: -10,
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "CurrentHealthy: -5",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				CurrentHealthy: -5,
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "DesiredHealthy: -3",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				DesiredHealthy: -3,
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "ExpectedPods: -2",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				ExpectedPods: -2,
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "Conditions valid",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				Conditions: []metav1.Condition{
					{
						Type:   policyv1beta1.DisruptionAllowedCondition,
						Status: metav1.ConditionTrue,
						LastTransitionTime: metav1.Time{
							Time: time.Now().Add(-5 * time.Minute),
						},
						Reason:             policyv1beta1.SufficientPodsReason,
						Message:            "message",
						ObservedGeneration: 3,
					},
				},
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectNoErrors,
				policyv1beta1.SchemeGroupVersion: expectNoErrors,
			},
		},
		{
			name: "Conditions not valid",
			pdbStatus: policy.PodDisruptionBudgetStatus{
				Conditions: []metav1.Condition{
					{
						Type:   policyv1beta1.DisruptionAllowedCondition,
						Status: metav1.ConditionTrue,
					},
					{
						Type:   policyv1beta1.DisruptionAllowedCondition,
						Status: metav1.ConditionFalse,
					},
				},
			},
			expectErrForVersion: map[schema.GroupVersion]bool{
				policy.SchemeGroupVersion:        expectErrors,
				policyv1beta1.SchemeGroupVersion: expectErrors,
			},
		},
	}

	for _, tc := range testCases {
		for apiVersion, expectErrors := range tc.expectErrForVersion {
			t.Run(fmt.Sprintf("apiVersion: %s, %s", apiVersion.String(), tc.name), func(t *testing.T) {
				errors := ValidatePodDisruptionBudgetStatusUpdate(tc.pdbStatus, policy.PodDisruptionBudgetStatus{},
					field.NewPath("status"), apiVersion)
				errCount := len(errors)

				if errCount > 0 && !expectErrors {
					t.Errorf("unexpected failure %v for %v", errors, tc.pdbStatus)
				}

				if errCount == 0 && expectErrors {
					t.Errorf("expected errors but didn't one for %v", tc.pdbStatus)
				}
			})
		}
	}
}

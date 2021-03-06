package helper

import (
	"context"
	"fmt"
	"testing"
	"time"

	fakeworkclient "github.com/open-cluster-management/api/client/work/clientset/versioned/fake"
	workapiv1 "github.com/open-cluster-management/api/work/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
)

func newCondition(name, status, reason, message string, lastTransition *metav1.Time) workapiv1.StatusCondition {
	ret := workapiv1.StatusCondition{
		Type:    name,
		Status:  metav1.ConditionStatus(status),
		Reason:  reason,
		Message: message,
	}
	if lastTransition != nil {
		ret.LastTransitionTime = *lastTransition
	}
	return ret
}

func updateSpokeClusterConditionFn(cond workapiv1.StatusCondition) UpdateManifestWorkStatusFunc {
	return func(oldStatus *workapiv1.ManifestWorkStatus) error {
		SetStatusCondition(&oldStatus.Conditions, cond)
		return nil
	}
}

func newManifestCondition(ordinal int32, resource string, conds ...workapiv1.StatusCondition) workapiv1.ManifestCondition {
	return workapiv1.ManifestCondition{
		ResourceMeta: workapiv1.ManifestResourceMeta{Ordinal: ordinal, Resource: resource},
		Conditions:   conds,
	}
}

// TestUpdateStatusCondition tests UpdateManifestWorkStatus function
func TestUpdateStatusCondition(t *testing.T) {
	nowish := metav1.Now()
	beforeish := metav1.Time{Time: nowish.Add(-10 * time.Second)}
	afterish := metav1.Time{Time: nowish.Add(10 * time.Second)}

	cases := []struct {
		name               string
		startingConditions []workapiv1.StatusCondition
		newCondition       workapiv1.StatusCondition
		expectedUpdated    bool
		expectedConditions []workapiv1.StatusCondition
	}{
		{
			name:               "add to empty",
			startingConditions: []workapiv1.StatusCondition{},
			newCondition:       newCondition("test", "True", "my-reason", "my-message", nil),
			expectedUpdated:    true,
			expectedConditions: []workapiv1.StatusCondition{newCondition("test", "True", "my-reason", "my-message", nil)},
		},
		{
			name: "add to non-conflicting",
			startingConditions: []workapiv1.StatusCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
			},
			newCondition:    newCondition("one", "True", "my-reason", "my-message", nil),
			expectedUpdated: true,
			expectedConditions: []workapiv1.StatusCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", nil),
			},
		},
		{
			name: "change existing status",
			startingConditions: []workapiv1.StatusCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", nil),
			},
			newCondition:    newCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			expectedUpdated: true,
			expectedConditions: []workapiv1.StatusCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			},
		},
		{
			name: "leave existing transition time",
			startingConditions: []workapiv1.StatusCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
			newCondition:    newCondition("one", "True", "my-reason", "my-message", &afterish),
			expectedUpdated: false,
			expectedConditions: []workapiv1.StatusCondition{
				newCondition("two", "True", "my-reason", "my-message", nil),
				newCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeWorkClient := fakeworkclient.NewSimpleClientset(&workapiv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{Name: "work1", Namespace: "cluster1"},
				Status: workapiv1.ManifestWorkStatus{
					Conditions: c.startingConditions,
				},
			})

			status, updated, err := UpdateManifestWorkStatus(
				context.TODO(),
				fakeWorkClient.WorkV1().ManifestWorks("cluster1"),
				"work1",
				updateSpokeClusterConditionFn(c.newCondition),
			)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if updated != c.expectedUpdated {
				t.Errorf("expected %t, but %t", c.expectedUpdated, updated)
			}
			for i := range c.expectedConditions {
				expected := c.expectedConditions[i]
				actual := status.Conditions[i]
				if expected.LastTransitionTime == (metav1.Time{}) {
					actual.LastTransitionTime = metav1.Time{}
				}
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Errorf(diff.ObjectDiff(expected, actual))
				}
			}
		})
	}
}

// TestSetManifestCondition tests SetManifestCondition function
func TestSetManifestCondition(t *testing.T) {
	cases := []struct {
		name               string
		startingConditions []workapiv1.ManifestCondition
		newCondition       workapiv1.ManifestCondition
		expectedConditions []workapiv1.ManifestCondition
	}{
		{
			name:               "add to empty",
			startingConditions: []workapiv1.ManifestCondition{},
			newCondition:       newManifestCondition(0, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
			expectedConditions: []workapiv1.ManifestCondition{
				newManifestCondition(0, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
			},
		},
		{
			name: "add new conddtion",
			startingConditions: []workapiv1.ManifestCondition{
				newManifestCondition(0, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
			},
			newCondition: newManifestCondition(1, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
			expectedConditions: []workapiv1.ManifestCondition{
				newManifestCondition(0, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
				newManifestCondition(1, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
			},
		},
		{
			name: "update existing",
			startingConditions: []workapiv1.ManifestCondition{
				newManifestCondition(2, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
				newManifestCondition(1, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
			},
			newCondition: newManifestCondition(1, "resource2", newCondition("two", "True", "my-reason", "my-message", nil)),
			expectedConditions: []workapiv1.ManifestCondition{
				newManifestCondition(2, "resource1", newCondition("one", "True", "my-reason", "my-message", nil)),
				newManifestCondition(1, "resource2", newCondition("two", "True", "my-reason", "my-message", nil)),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := c.startingConditions
			SetManifestCondition(&actual, c.newCondition)
			fmt.Printf("found con %v\n", actual)
			if !equality.Semantic.DeepEqual(actual, c.expectedConditions) {
				t.Errorf(diff.ObjectDiff(actual, c.expectedConditions))
			}
		})
	}
}

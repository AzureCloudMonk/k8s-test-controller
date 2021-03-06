package run

import (
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	wait "k8s.io/apimachinery/pkg/util/wait"

	v1alpha1 "github.com/srossross/k8s-test-controller/pkg/apis/tester/v1alpha1"
	controller "github.com/srossross/k8s-test-controller/pkg/controller"
)

var (
	// APIVersion FIXME: not sure why this is needed
	APIVersion = "srossross.github.io/v1alpha1"

	// TestRunKind FIXME: not sure why this is needed
	TestRunKind = "TestRun"
)

func mergeMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func getTestOwnerReference(testRun *v1alpha1.TestRun) metav1.OwnerReference {
	Controller := true
	return metav1.OwnerReference{
		// FIXME: not sure why testRun.Kind is empty
		Kind: TestRunKind,
		Name: testRun.Name,
		UID:  testRun.UID,
		// FIXME: not sure why testRun.APIVersion is empty
		APIVersion: APIVersion,
		Controller: &Controller,
	}
}

// CreateTestPod creates a test pod from a test template
func CreateTestPod(ctrl controller.Interface, testRun *v1alpha1.TestRun, test *v1alpha1.TestTemplate) (*v1.Pod, error) {

	Namespace := testRun.Namespace
	if len(Namespace) == 0 {
		Namespace = "default"
	}

	err := CreateTestRunEventStart(ctrl, testRun, test)
	if err != nil {
		return nil, err
	}

	Annotations := mergeMaps(test.Spec.Template.Annotations, map[string]string{
		"srossross.github.io/v1alpha1": fmt.Sprintf("TestRun:%v/%v", testRun.Namespace, testRun.Name),
	})
	Labels := mergeMaps(test.Spec.Template.Labels, map[string]string{
		"test-run":  testRun.Name,
		"test-name": test.Name,
	})

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", test.Name),
			Namespace:    Namespace,
			Annotations:  Annotations,
			Labels:       Labels,
			OwnerReferences: []metav1.OwnerReference{
				getTestOwnerReference(testRun),
			},
		},
		Spec:   test.Spec.Template.Spec,
		Status: v1.PodStatus{},
	}

	createdPod, err := ctrl.CreatePod(Namespace, pod)
	if err != nil {
		CreateTestRunEvent(
			ctrl, testRun, test.Name, "PodCreationFailure",
			fmt.Sprintf("Could not create pod for test %s", test.Name),
		)
		log.Printf("Error Creating pod while starting test %v", err)

		return nil, err
	}
	log.Printf("  |  Test created pod '%s/%s'", Namespace, createdPod.Name)

	return createdPod, wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {

		_, err := ctrl.GetPod(testRun.Namespace, createdPod.Name)

		if err == nil {
			return true, nil
		}

		if errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	})
}

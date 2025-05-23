/*
Copyright 2019 The Volcano Authors.

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

package podgroup

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes/fake"

	scheduling "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	vcclient "volcano.sh/apis/pkg/client/clientset/versioned/fake"
	informerfactory "volcano.sh/apis/pkg/client/informers/externalversions"
	"volcano.sh/volcano/pkg/controllers/framework"
)

func newFakeController() *pgcontroller {
	kubeClient := kubeclient.NewSimpleClientset()
	vcClient := vcclient.NewSimpleClientset()
	sharedInformers := informers.NewSharedInformerFactory(kubeClient, 0)
	vcSharedInformers := informerfactory.NewSharedInformerFactory(vcClient, 0)

	controller := &pgcontroller{}
	opt := &framework.ControllerOption{
		KubeClient:              kubeClient,
		VolcanoClient:           vcClient,
		SharedInformerFactory:   sharedInformers,
		VCSharedInformerFactory: vcSharedInformers,
		SchedulerNames:          []string{"volcano"},
		InheritOwnerAnnotations: true,
	}

	controller.Initialize(opt)

	return controller
}

func TestAddPodGroup(t *testing.T) {
	namespace := "test"
	isController := true
	blockOwnerDeletion := true
	replicas := int32(2)
	gpuKey := v1.ResourceName("nvidia.com/gpu")

	testCases := []struct {
		name             string
		rs               *appsv1.ReplicaSet
		pods             []*v1.Pod
		expectedPodGroup *scheduling.PodGroup
	}{
		{
			name: "AddPodGroup: pod has ownerReferences and priorityClassName",
			pods: []*v1.Pod{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "app/v1",
								Kind:       "ReplicaSet",
								Name:       "rs1",
								UID:        "7a09885b-b753-4924-9fba-77c0836bac20",
								Controller: &isController,
							},
						},
					},
					Spec: v1.PodSpec{
						PriorityClassName: "test-pc",
					},
				},
			},
			expectedPodGroup: &scheduling.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "scheduling.volcano.sh/v1beta1",
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podgroup-7a09885b-b753-4924-9fba-77c0836bac20",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "app/v1",
							Kind:       "ReplicaSet",
							Name:       "rs1",
							UID:        "7a09885b-b753-4924-9fba-77c0836bac20",
							Controller: &isController,
						},
					},
				},
				Spec: scheduling.PodGroupSpec{
					MinMember:         1,
					PriorityClassName: "test-pc",
				},
			},
		},
		{
			name: "AddPodGroup: pod has no ownerReferences or priorityClassName",
			pods: []*v1.Pod{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: namespace,
						UID:       types.UID("7a09885b-b753-4924-9fba-77c0836bac20"),
					},
				},
			},
			expectedPodGroup: &scheduling.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "scheduling.volcano.sh/v1beta1",
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podgroup-7a09885b-b753-4924-9fba-77c0836bac20",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "v1",
							Kind:               "Pod",
							Name:               "pod1",
							UID:                "7a09885b-b753-4924-9fba-77c0836bac20",
							Controller:         &isController,
							BlockOwnerDeletion: &blockOwnerDeletion,
						},
					},
				},
				Spec: scheduling.PodGroupSpec{
					MinMember: 1,
				},
			},
		},
		{
			name: "AddPodGroup: pod owners with group-min-member annotation",
			rs: &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rs1",
					Namespace: namespace,
					UID:       "7a09885b-b753-4924-9fba-77c0836bac20",
					Annotations: map[string]string{
						scheduling.VolcanoGroupMinMemberAnnotationKey: "2",
					},
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ReplicaSet",
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "rs1",
						},
					},
					Replicas: &replicas,
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container1",
									Resources: v1.ResourceRequirements{
										Requests: v1.ResourceList{
											gpuKey: resource.MustParse("1"),
										},
									},
								},
							},
						},
					},
				},
			},
			pods: []*v1.Pod{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: namespace,
						Labels: map[string]string{
							"app": "rs1",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "app/v1",
								Kind:       "ReplicaSet",
								Name:       "rs1",
								UID:        "7a09885b-b753-4924-9fba-77c0836bac20",
								Controller: &isController,
							},
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name: "container1",
								Resources: v1.ResourceRequirements{
									Requests: v1.ResourceList{
										gpuKey: resource.MustParse("1"),
									},
								},
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod2",
						Namespace: namespace,
						Labels: map[string]string{
							"app": "rs1",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "app/v1",
								Kind:       "ReplicaSet",
								Name:       "rs1",
								UID:        "7a09885b-b753-4924-9fba-77c0836bac20",
								Controller: &isController,
							},
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name: "container1",
								Resources: v1.ResourceRequirements{
									Requests: v1.ResourceList{
										gpuKey: resource.MustParse("1"),
									},
								},
							},
						},
					},
				},
			},
			expectedPodGroup: &scheduling.PodGroup{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "scheduling.volcano.sh/v1beta1",
					Kind:       "PodGroup",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podgroup-7a09885b-b753-4924-9fba-77c0836bac20",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "app/v1",
							Kind:       "ReplicaSet",
							Name:       "rs1",
							UID:        "7a09885b-b753-4924-9fba-77c0836bac20",
							Controller: &isController,
						},
					},
				},
				Spec: scheduling.PodGroupSpec{
					MinMember: 2,
					MinResources: &v1.ResourceList{
						gpuKey: resource.MustParse("2"),
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		c := newFakeController()

		if testCase.rs != nil {
			rs, err := c.kubeClient.AppsV1().ReplicaSets(namespace).Create(context.TODO(), testCase.rs, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Case %s failed when creating replicaSet for %v", testCase.name, err)
			}

			c.addReplicaSet(rs)
		}

		for i := range testCase.pods {
			pod, err := c.kubeClient.CoreV1().Pods(testCase.pods[i].Namespace).Create(context.TODO(), testCase.pods[i], metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Case %s failed when creating pod for %v", testCase.name, err)
			}

			c.addPod(pod)
			c.createNormalPodPGIfNotExist(pod)

			pg, err := c.vcClient.SchedulingV1beta1().PodGroups(pod.Namespace).Get(context.TODO(),
				testCase.expectedPodGroup.Name,
				metav1.GetOptions{},
			)
			if err != nil {
				t.Errorf("Case %s failed when getting podGroup for %v", testCase.name, err)
			}

			if false == equality.Semantic.DeepEqual(pg.OwnerReferences, testCase.expectedPodGroup.OwnerReferences) {
				t.Errorf("Case %s failed, expect %v, got %v", testCase.name, testCase.expectedPodGroup, pg)
			}

			newpod, err := c.kubeClient.CoreV1().Pods(testCase.pods[i].Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Case %s failed when creating pod for %v", testCase.name, err)
			}

			podAnnotation := newpod.Annotations[scheduling.KubeGroupNameAnnotationKey]
			if testCase.expectedPodGroup.Name != podAnnotation {
				t.Errorf("Case %s failed, expect %v, got %v", testCase.name,
					testCase.expectedPodGroup.Name, podAnnotation)
			}

			if testCase.expectedPodGroup.Spec.PriorityClassName != pod.Spec.PriorityClassName {
				t.Errorf("Case %s failed, expect %v, got %v", testCase.name,
					testCase.expectedPodGroup.Spec.PriorityClassName, pod.Spec.PriorityClassName)
			}

			if pg.Spec.MinMember != testCase.expectedPodGroup.Spec.MinMember {
				t.Errorf("Case %s failed, expect %v, got %v", testCase.name, testCase.expectedPodGroup.Spec.MinMember, pg.Spec.MinMember)
			}

			if testCase.expectedPodGroup.Spec.MinResources != nil && false == equality.Semantic.DeepEqual(pg.Spec.MinResources.Name(gpuKey, resource.DecimalSI), testCase.expectedPodGroup.Spec.MinResources.Name(gpuKey, resource.DecimalSI)) {
				t.Errorf("Case %s failed, expect %v, got %v", testCase.name, testCase.expectedPodGroup.Spec.MinResources.Name(gpuKey, resource.DecimalSI), pg.Spec.MinResources.Name(gpuKey, resource.DecimalSI))
			}
		}
	}
}

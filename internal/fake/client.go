/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	"context"
	"reflect"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func NewClientWithObjects(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme))
	utilruntime.Must(gardensecurityv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(seedmanagementv1alpha1.AddToScheme(scheme))
	utilruntime.Must(machinev1alpha1.AddToScheme(scheme))

	return Wrap(fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build())
}

func Wrap(client client.Client) client.Client {
	return &clientWrapper{delegate: client}
}

type clientWrapper struct {
	delegate client.Client
}

var _ client.Client = &clientWrapper{}

func (w *clientWrapper) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return w.delegate.Get(ctx, key, obj, opts...)
}

func (w *clientWrapper) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	o := &client.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(o)
	}

	fieldSelector := o.FieldSelector
	o.FieldSelector = nil

	err := w.delegate.List(ctx, list, o)
	if err != nil {
		return err
	}

	if fieldSelector == nil || fieldSelector.Empty() {
		return nil
	}

	switch list := list.(type) {
	case *gardencorev1beta1.ShootList:
		filterItems(list, fieldSelector, func(vItem reflect.Value) fields.Set {
			fieldSet := fields.Set{}
			vName := vItem.FieldByName("Name")
			fieldSet["metadata.name"] = vName.String()
			vSpec := vItem.FieldByName("Spec")
			vSeedName := vSpec.FieldByName("SeedName")

			if !vSeedName.IsNil() {
				fieldSet[gardencore.ShootSeedName] = reflect.Indirect(vSeedName).String()
			}

			return fieldSet
		})
	case *gardencorev1beta1.ProjectList:
		filterItems(list, fieldSelector, func(vItem reflect.Value) fields.Set {
			fieldSet := fields.Set{}
			vName := vItem.FieldByName("Name")
			fieldSet["metadata.name"] = vName.String()
			vSpec := vItem.FieldByName("Spec")
			vNamespace := vSpec.FieldByName("Namespace")

			if !vNamespace.IsNil() {
				fieldSet[gardencore.ProjectNamespace] = reflect.Indirect(vNamespace).String()
			}

			return fieldSet
		})
	}

	return nil
}

func (w *clientWrapper) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return w.delegate.Create(ctx, obj, opts...)
}

func (w *clientWrapper) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return w.delegate.Delete(ctx, obj, opts...)
}

func (w *clientWrapper) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return w.delegate.Update(ctx, obj, opts...)
}

func (w *clientWrapper) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return w.delegate.Patch(ctx, obj, patch, opts...)
}

func (w *clientWrapper) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	return w.delegate.Apply(ctx, obj, opts...)
}

func (w *clientWrapper) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return w.delegate.DeleteAllOf(ctx, obj, opts...)
}

func (w *clientWrapper) Status() client.StatusWriter {
	return w.delegate.Status()
}

func (w *clientWrapper) Scheme() *runtime.Scheme {
	return w.delegate.Scheme()
}

func (w *clientWrapper) RESTMapper() meta.RESTMapper {
	return w.delegate.RESTMapper()
}

func (w *clientWrapper) SubResource(subResource string) client.SubResourceClient {
	return w.delegate.SubResource(subResource)
}

func (w *clientWrapper) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return w.delegate.GroupVersionKindFor(obj)
}

func (w *clientWrapper) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return w.delegate.IsObjectNamespaced(obj)
}

func filterItems(list client.ObjectList, selector fields.Selector, getFieldSet func(reflect.Value) fields.Set) {
	vList := reflect.Indirect(reflect.ValueOf(list))
	vItems := vList.FieldByName("Items")
	vFilteredItems := reflect.MakeSlice(vItems.Type(), 0, vItems.Len())

	for i := 0; i < vItems.Len(); i++ {
		vItem := vItems.Index(i)
		fieldSet := getFieldSet(vItem)

		if selector.Matches(fieldSet) {
			vFilteredItems = reflect.Append(vFilteredItems, vItem)
		}
	}

	vItems.Set(vFilteredItems)
}

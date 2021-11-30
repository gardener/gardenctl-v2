/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package fake

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
)

func Wrap(client client.Client) client.Client {
	return &clientWrapper{delegate: client}
}

type clientWrapper struct {
	delegate client.Client
}

var _ client.Client = &clientWrapper{}

func (w *clientWrapper) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	return w.delegate.Get(ctx, key, obj)
}

func (w *clientWrapper) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	err := w.delegate.List(ctx, list, opts...)

	fieldSelector := getFieldSelector(opts...)
	if !fieldSelector.Empty() {
		gvk := list.GetObjectKind().GroupVersionKind()
		apiVersion, kind := gvk.ToAPIVersionAndKind()

		if apiVersion == "core.gardener.cloud/v1beta1" {
			switch kind {
			case "ShootList":
				filterShootItems(list, fieldSelector)
			case "ProjectList":
				filterProjectItems(list, fieldSelector)
			}
		}
	}

	return err
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

func getFieldSelector(opts ...client.ListOption) fields.Selector {
	fieldSelectors := []fields.Selector{}

	for _, opt := range opts {
		o := &client.ListOptions{}
		opt.ApplyToList(o)

		if o.FieldSelector != nil && !o.FieldSelector.Empty() {
			fieldSelectors = append(fieldSelectors, o.FieldSelector)
		}
	}

	return fields.AndSelectors(fieldSelectors...)
}

func filterProjectItems(list client.ObjectList, selector fields.Selector) {
	vList := reflect.Indirect(reflect.ValueOf(list))
	vItems := vList.FieldByName("Items")
	items := reflect.MakeSlice(vItems.Type(), 0, vItems.Len())

	for i := 0; i < vItems.Len(); i++ {
		vItem := vItems.Index(i)
		vName := vItem.FieldByName("Name")
		set := fields.Set{"metadata.name": vName.String()}
		vSpec := vItem.FieldByName("Spec")
		vNamespace := vSpec.FieldByName("Namespace")

		if !vNamespace.IsNil() {
			set[gardencore.ProjectNamespace] = reflect.Indirect(vNamespace).String()
		}

		if selector.Matches(set) {
			items = reflect.Append(items, vItem)
		}
	}

	vItems.Set(items)
}

func filterShootItems(list client.ObjectList, selector fields.Selector) {
	vList := reflect.Indirect(reflect.ValueOf(list))
	vItems := vList.FieldByName("Items")
	items := reflect.MakeSlice(vItems.Type(), 0, vItems.Len())

	for i := 0; i < vItems.Len(); i++ {
		vItem := vItems.Index(i)
		vName := vItem.FieldByName("Name")
		set := fields.Set{"metadata.name": vName.String()}
		vSpec := vItem.FieldByName("Spec")
		vSeedName := vSpec.FieldByName("SeedName")

		if !vSeedName.IsNil() {
			set[gardencore.ShootSeedName] = reflect.Indirect(vSeedName).String()
		}

		if selector.Matches(set) {
			items = reflect.Append(items, vItem)
		}
	}

	vItems.Set(items)
}

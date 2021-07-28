package client

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type statusWriter struct {
	client *Client
}

func (sw *statusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	newStatus := &unstructured.Unstructured{}
	if err := Convert(newStatus, obj); err != nil {
		return err
	}

	gvk, err := sw.client.gvk(obj)
	if err != nil {
		return err
	}

	existing := sw.client.store.Get(gvk, obj.GetNamespace(), obj.GetName())
	if existing == nil {
		return errors.NewNotFound(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, obj.GetName())
	}

	existing = existing.DeepCopyObject().(client.Object)
	existing.SetResourceVersion(newStatus.GetResourceVersion())
	existing.(*unstructured.Unstructured).Object["status"] = newStatus.Object["status"]

	ret, err := sw.client.store.Update(ctx, gvk, existing, false)
	if err != nil {
		return err
	}

	return Convert(obj, ret)
}

func (sw *statusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return sw.client.patch(ctx, obj, patch, false, opts...)
}

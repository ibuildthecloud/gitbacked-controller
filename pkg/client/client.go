package client

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ibuildthecloud/gitbacked-controller/pkg/store"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type Client struct {
	scheme *runtime.Scheme
	mapper meta.RESTMapper
	store  *store.Store
}

func NewClient(scheme *runtime.Scheme, mapper meta.RESTMapper, store *store.Store) *Client {
	return &Client{
		scheme: scheme,
		mapper: mapper,
		store:  store,
	}
}

func Convert(to, from interface{}) error {
	return store.Convert(to, from)
}

func (c *Client) gvk(obj runtime.Object) (schema.GroupVersionKind, error) {
	return GVK(c.scheme, obj)
}

func trimList(gvk schema.GroupVersionKind) schema.GroupVersionKind {
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")
	return gvk

}

func GVK(scheme *runtime.Scheme, obj runtime.Object) (schema.GroupVersionKind, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Kind != "" {
		return trimList(gvk), nil
	}
	gvk, err := apiutil.GVKForObject(obj, scheme)
	return trimList(gvk), err
}

func (c *Client) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	gvk, err := c.gvk(obj)
	if err != nil {
		return err
	}

	ret := c.store.Get(gvk, key.Namespace, key.Name)
	if ret == nil {
		return errors.NewNotFound(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, key.Name)
	}

	return Convert(obj, ret)
}

func (c *Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	gvk, err := c.gvk(list)
	if err != nil {
		return err
	}

	listOpts := client.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(&listOpts)
	}
	retList := c.store.List(gvk, listOpts.Namespace, listOpts.LabelSelector)
	return Convert(list, retList)
}

func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	gvk, err := c.gvk(obj)
	if err != nil {
		return err
	}

	ret, err := c.store.Create(ctx, gvk, obj)
	if err != nil {
		return err
	}
	return Convert(obj, ret)
}

func (c *Client) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	gvk, err := c.gvk(obj)
	if err != nil {
		return err
	}

	deleteOptions := client.DeleteOptions{}
	for _, opt := range opts {
		opt.ApplyToDelete(&deleteOptions)
	}
	return c.store.Delete(ctx, gvk, obj.GetNamespace(), obj.GetName(), deleteOptions.Preconditions)
}

func (c *Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	gvk, err := c.gvk(obj)
	if err != nil {
		return err
	}

	ret, err := c.store.Update(ctx, gvk, obj, true)
	if err != nil {
		return err
	}
	return Convert(obj, ret)
}

func (c *Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.patch(ctx, obj, patch, true, opts...)
}

func (c *Client) patch(ctx context.Context, obj client.Object, patch client.Patch, generation bool, opts ...client.PatchOption) error {
	gvk, err := c.gvk(obj)
	if err != nil {
		return err
	}

	originalObj := c.store.Get(gvk, obj.GetNamespace(), obj.GetName())
	if originalObj == nil {
		return errors.NewNotFound(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, obj.GetName())
	}

	patchBytes, err := patch.Data(originalObj)
	if err != nil {
		return err
	}

	newBytes, err := apply(gvk, c.scheme, originalObj, patchBytes, patch.Type())
	if err != nil {
		return err
	}

	newObj := map[string]interface{}{}
	if err := json.Unmarshal(newBytes, &newObj); err != nil {
		return err
	}

	ret, err := c.store.Update(ctx, gvk, &unstructured.Unstructured{
		Object: newObj,
	}, generation)
	if err != nil {
		return err
	}

	return Convert(obj, ret)
}

func (c *Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

func (c *Client) Watch(gvk schema.GroupVersionKind, emptyObj client.Object, opts metav1.ListOptions) (watch.Interface, error) {
	return c.store.Watch(gvk, emptyObj, opts)
}

func (c *Client) Status() client.StatusWriter {
	return &statusWriter{client: c}
}

func (c *Client) Scheme() *runtime.Scheme {
	return c.scheme
}

func (c *Client) RESTMapper() meta.RESTMapper {
	return nil
}

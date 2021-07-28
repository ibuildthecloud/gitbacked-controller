package cache

import (
	"context"

	client2 "github.com/ibuildthecloud/gitbacked-controller/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type list struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []runtime.Object `json:"items"`
}

func (l *list) DeepCopyObject() runtime.Object {
	newList := list{
		TypeMeta: l.TypeMeta,
		ListMeta: *l.ListMeta.DeepCopy(),
		Items:    make([]runtime.Object, len(l.Items)),
	}
	for i := range l.Items {
		newList.Items[i] = l.Items[i].DeepCopyObject()
	}
	return &newList
}

func (c *Cache) newListWatch(gvk schema.GroupVersionKind, emptyObj client.Object) (*cache.ListWatch, error) {
	apiVersion, listKind := gvk.ToAPIVersionAndKind()
	listKind = listKind + "List"

	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			uList := &unstructured.UnstructuredList{}
			uList.SetKind(listKind)
			uList.SetAPIVersion(apiVersion)

			if err := c.client.List(context.Background(), uList); err != nil {
				return nil, err
			}

			list := &list{}
			for _, obj := range uList.Items {
				newObj := emptyObj.DeepCopyObject()
				if err := client2.Convert(newObj, &obj); err != nil {
					return nil, err
				}
				list.Items = append(list.Items, newObj)
			}
			list.SetResourceVersion(uList.GetResourceVersion())
			return list, nil
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return c.client.Watch(gvk, emptyObj, opts)
		},
	}, nil
}

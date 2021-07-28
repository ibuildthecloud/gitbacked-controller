package cache

import (
	"context"
	"sync"
	"time"

	client2 "github.com/ibuildthecloud/gitbacked-controller/pkg/client"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cache2 "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Cache struct {
	lock sync.Mutex

	ctx       context.Context
	informers map[runtime.Object]cache2.SharedIndexInformer
	started   map[cache2.SharedIndexInformer]bool
	client    *client2.Client
}

func New(client *client2.Client) *Cache {
	return &Cache{
		informers: map[runtime.Object]cache2.SharedIndexInformer{},
		started:   map[cache2.SharedIndexInformer]bool{},
		client:    client,
	}
}

func (c *Cache) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	gvk, err := client2.GVK(c.client.Scheme(), obj)
	if err != nil {
		return err
	}

	informer, started, err := c.getInformer(ctx, obj)
	if err != nil {
		return err
	}

	if !started {
		return &cache.ErrCacheNotStarted{}
	}

	found, exist, err := informer.GetStore().GetByKey(key.String())
	if err != nil {
		return err
	} else if !exist {
		return errors.NewNotFound(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, key.String())
	}

	return client2.Convert(obj, found)
}

func (c *Cache) List(ctx context.Context, listObj client.ObjectList, opts ...client.ListOption) error {
	listOptions := client.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(&listOptions)
	}

	gvk, err := client2.GVK(c.client.Scheme(), listObj)
	if err != nil {
		return err
	}

	gvkObj := c.objectForGVK(gvk)

	informer, started, err := c.getInformer(ctx, gvkObj)
	if err != nil {
		return err
	}

	if !started {
		return &cache.ErrCacheNotStarted{}
	}

	retList := &list{}
	retList.APIVersion, retList.Kind = gvk.ToAPIVersionAndKind()
	retList.Kind = retList.Kind + "List"

	for _, listObj := range informer.GetStore().List() {
		obj := listObj.(client.Object)
		if listOptions.Namespace != "" && obj.GetNamespace() != listOptions.Namespace {
			continue
		}
		if listOptions.LabelSelector != nil && !listOptions.LabelSelector.Matches(labels.Set(obj.GetLabels())) {
			continue
		}
		retList.Items = append(retList.Items, obj)
	}

	return client2.Convert(listObj, retList)
}

func (c *Cache) objectForGVK(gvk schema.GroupVersionKind) client.Object {
	emptyObj, err := c.client.Scheme().New(gvk)
	if err != nil {
		emptyObj = &unstructured.Unstructured{}
	}
	return emptyObj.(client.Object)
}

func (c *Cache) newInformer(ctx context.Context, gvk schema.GroupVersionKind, obj client.Object) (cache2.SharedIndexInformer, error) {
	lw, err := c.newListWatch(gvk, obj)
	if err != nil {
		return nil, err
	}
	informer := cache2.NewSharedIndexInformer(lw, obj, 2*time.Minute, cache2.Indexers{
		cache2.NamespaceIndex: cache2.MetaNamespaceIndexFunc,
	})
	if c.ctx != nil {
		go informer.Run(c.ctx.Done())
		c.started[informer] = true
		cache2.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	}
	return informer, nil
}

func (c *Cache) GetInformer(ctx context.Context, obj client.Object) (cache.Informer, error) {
	inf, _, err := c.getInformer(ctx, obj)
	return inf, err
}

func (c *Cache) getInformer(ctx context.Context, obj client.Object) (cache2.SharedIndexInformer, bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	gvk, err := client2.GVK(c.client.Scheme(), obj)
	if err != nil {
		return nil, c.ctx != nil, err
	}

	emptyObj := c.objectForGVK(gvk)

	informer := c.informers[emptyObj]
	if informer != nil {
		return informer, c.ctx != nil, nil
	}

	informer, err = c.newInformer(ctx, gvk, emptyObj)
	if err != nil {
		return nil, c.ctx != nil, err
	}

	c.informers[emptyObj] = informer
	return informer, c.ctx != nil, nil
}

func (c *Cache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind) (cache.Informer, error) {
	return c.GetInformer(ctx, c.objectForGVK(gvk))
}

func (c *Cache) Start(ctx context.Context) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.ctx != nil {
		return nil
	}

	for _, informer := range c.informers {
		if !c.started[informer] {
			go informer.Run(ctx.Done())
			c.started[informer] = true
		}
	}
	c.ctx = ctx
	return nil
}

func (c *Cache) WaitForCacheSync(ctx context.Context) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	for informer := range c.started {
		if !cache2.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
			return false
		}
	}

	return true
}

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func (s *Store) Get(gvk schema.GroupVersionKind, namespace, name string) client.Object {
	s.contentLock.RLock()
	defer s.contentLock.RUnlock()
	return s.get(gvk, namespace, name).Object
}

func (s *Store) get(gvk schema.GroupVersionKind, namespace, name string) Object {
	index := len(s.revisions) - 1
	rev := s.revisions[index]
	for key, obj := range rev.data {
		if key.Kind == gvk.Kind &&
			key.Group == gvk.Group &&
			key.Namespace == namespace &&
			key.Name == name {
			return obj
		}
	}

	return Object{}
}

func (s *Store) List(gvk schema.GroupVersionKind, namespace string, selector labels.Selector) runtime.Object {
	s.contentLock.RLock()
	defer s.contentLock.RUnlock()

	var items []runtime.Object

	index := len(s.revisions) - 1
	rev := s.revisions[index]
	for key, obj := range rev.data {
		if key.Kind == gvk.Kind &&
			key.Group == gvk.Group &&
			(namespace == "" || obj.Namespace == namespace) &&
			(selector == nil || selector.Matches(labels.Set(obj.Object.GetLabels()))) {
			items = append(items, obj.Object)
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":  gvk.Kind + "List",
			"items": items,
			"metadata": map[string]interface{}{
				"resourceVersion": strconv.Itoa(index),
			},
		},
	}
}

func (s *Store) Create(ctx context.Context, gvk schema.GroupVersionKind, object client.Object) (runtime.Object, error) {
	s.contentLock.Lock()
	defer s.contentLock.Unlock()

	name := object.GetName()
	namespace := object.GetNamespace()

	if name == "" && object.GetGenerateName() != "" {
		prefix := object.GetGenerateName()
		if len(prefix) > 58 {
			prefix = prefix[:58]
		}
		for {
			testName := fmt.Sprintf("%s%s", prefix, rand.String(4))
			existing := s.get(gvk, namespace, testName)
			if existing.Object == nil {
				name = testName
				object = object.DeepCopyObject().(client.Object)
				object.SetName(name)
				break
			}
		}
	}

	existing := s.get(gvk, namespace, name)
	if existing.Object != nil {
		return nil, errors.NewAlreadyExists(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, name)
	}

	file := filepath.Join(s.repo.Dir, s.subDir, gvk.Group, gvk.Version, gvk.Kind, namespace, name) + ".yaml"
	return s.save(ctx, gvk, object, file, true)
}

func (s *Store) save(ctx context.Context, gvk schema.GroupVersionKind, object client.Object, path string, generation bool) (runtime.Object, error) {
	cloned := object.DeepCopyObject()
	t, err := meta.TypeAccessor(cloned)
	if err != nil {
		return nil, err
	}
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	t.SetKind(kind)
	t.SetAPIVersion(apiVersion)

	if generation {
		meta, err := meta.Accessor(cloned)
		if err != nil {
			return nil, err
		}

		meta.SetGeneration(meta.GetGeneration() + 1)
	}

	data, err := yaml.Marshal(cloned)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Add(ctx, path, data); err != nil {
		return nil, err
	}

	if err := s.scanAndUpdate(); err != nil {
		return nil, err
	}

	return s.get(gvk, object.GetNamespace(), object.GetName()).Object, nil
}

func (s *Store) Delete(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string, preconditions *metav1.Preconditions) error {
	s.contentLock.RLock()
	defer s.contentLock.RUnlock()

	found := s.get(gvk, namespace, name)
	if found.Object == nil {
		return nil
	}

	meta, err := meta.Accessor(found.Object)
	if err != nil {
		return err
	}

	if preconditions != nil && preconditions.ResourceVersion != nil && meta.GetResourceVersion() != *preconditions.ResourceVersion {
		return errors.NewConflict(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, name, fmt.Errorf("resourceVersion %s does not match requested %s", meta.GetResourceVersion(), *preconditions.ResourceVersion))
	}

	if preconditions != nil && preconditions.UID != nil && meta.GetUID() != *preconditions.UID {
		return errors.NewConflict(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, name, fmt.Errorf("uid %s does not match requested %s", meta.GetUID(), *preconditions.UID))
	}

	if err := s.repo.Delete(ctx, found.Path); err != nil {
		return err
	}

	return s.scanAndUpdate()
}

func (s *Store) Update(ctx context.Context, gvk schema.GroupVersionKind, obj client.Object, generation bool) (runtime.Object, error) {
	s.contentLock.RLock()
	defer s.contentLock.RUnlock()

	found := s.get(gvk, obj.GetNamespace(), obj.GetName())
	if found.Object == nil {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, obj.GetName())
	}

	if obj.GetResourceVersion() != found.ResourceVersion {
		return nil, errors.NewConflict(schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}, obj.GetName(), fmt.Errorf("resourceVersion %s does not match requested %s", obj.GetResourceVersion(), found.ResourceVersion))
	}

	return s.save(ctx, gvk, obj, found.Path, generation)
}

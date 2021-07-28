package mapping

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Mapper struct {
}

func (m Mapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	panic("implement me")
}

func (m Mapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	panic("implement me")
}

func (m Mapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	panic("implement me")
}

func (m Mapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	panic("implement me")
}

func (m Mapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	if len(versions) == 0 {
		return nil, fmt.Errorf("one version is required")
	}
	return &meta.RESTMapping{
		Resource: schema.GroupVersionResource{
			Group:    gk.Group,
			Version:  versions[0],
			Resource: gk.Kind,
		},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   gk.Group,
			Version: versions[0],
			Kind:    gk.Kind,
		},
		Scope: meta.RESTScopeNamespace,
	}, nil
}

func (m Mapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	panic("implement me")
}

func (m Mapper) ResourceSingularizer(resource string) (singular string, err error) {
	panic("implement me")
}

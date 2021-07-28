package client

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func apply(gvk schema.GroupVersionKind, scheme *runtime.Scheme, obj client.Object, patch []byte, style types.PatchType) ([]byte, error) {
	original, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	switch style {
	case types.JSONPatchType:
		return applyJSONPatch(original, patch)
	case types.MergePatchType:
		return applyMergePatch(original, patch)
	case types.StrategicMergePatchType:
		return applyStrategicMergePatch(gvk, scheme, original, patch)
	default:
		return nil, fmt.Errorf("unsupported patch style: %v", style)
	}
}

func applyStrategicMergePatch(gvk schema.GroupVersionKind, scheme *runtime.Scheme, original, patch []byte) ([]byte, error) {
	versionedObject, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}

	lookup, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, err
	}
	originalMap := map[string]interface{}{}
	patchMap := map[string]interface{}{}
	if err := json.Unmarshal(original, &originalMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, err
	}
	patchedMap, err := strategicpatch.StrategicMergeMapPatchUsingLookupPatchMeta(originalMap, patchMap, lookup)
	if err != nil {
		return nil, err
	}
	return json.Marshal(patchedMap)
}

func applyMergePatch(original, patch []byte) ([]byte, error) {
	return jsonpatch.MergePatch(original, patch)
}

func applyJSONPatch(original, patch []byte) ([]byte, error) {
	jsonPatch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return nil, err
	}

	return jsonPatch.Apply(original)
}

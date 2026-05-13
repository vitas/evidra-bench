package scenario

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func manifestResources(path string) ([]resourceRef, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if strings.Contains(path, "argo-cd") && strings.Contains(path, "install.yaml") {
			return []resourceRef{
				{kind: "Namespace", name: "argocd"},
				{kind: "Deployment", namespace: "argocd", name: "argocd-server"},
				{kind: "Deployment", namespace: "argocd", name: "argocd-repo-server"},
				{kind: "StatefulSet", namespace: "argocd", name: "argocd-application-controller"},
				{kind: "CustomResourceDefinition", name: "applications.argoproj.io"},
			}, nil
		}
		return nil, fmt.Errorf("unsupported remote manifest path %q", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var refs []resourceRef
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			childRefs, err := manifestResources(filepath.Join(path, entry.Name()))
			if err != nil {
				return nil, err
			}
			refs = append(refs, childRefs...)
		}
		return refs, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(doc) == 0 {
			continue
		}
		kind, _ := doc["kind"].(string)
		metadata, _ := doc["metadata"].(map[string]any)
		name, _ := metadata["name"].(string)
		namespace, _ := metadata["namespace"].(string)
		if kind == "" || name == "" {
			continue
		}
		refs = append(refs, resourceRef{
			kind:      canonicalKind(kind),
			namespace: defaultNamespace(namespace),
			name:      name,
		})
	}
	return refs, nil
}

func deploymentContainerSets(path string) map[resourceRef]map[string]bool {
	result := map[resourceRef]map[string]bool{}
	objects, err := manifestObjects(path)
	if err != nil {
		return result
	}
	for _, obj := range objects {
		if obj.kind != "Deployment" {
			continue
		}
		if len(obj.containers) == 0 {
			continue
		}
		result[resourceRef{kind: obj.kind, namespace: defaultNamespace(obj.namespace), name: obj.name}] = obj.containers
	}
	return result
}

type manifestObject struct {
	kind       string
	namespace  string
	name       string
	containers map[string]bool
}

func manifestObjects(path string) ([]manifestObject, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return nil, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	var objects []manifestObject
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			child, err := manifestObjects(filepath.Join(path, entry.Name()))
			if err != nil {
				return nil, err
			}
			objects = append(objects, child...)
		}
		return objects, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(doc) == 0 {
			continue
		}
		kind, _ := doc["kind"].(string)
		metadata, _ := doc["metadata"].(map[string]any)
		name, _ := metadata["name"].(string)
		namespace, _ := metadata["namespace"].(string)
		if kind == "" || name == "" {
			continue
		}
		obj := manifestObject{
			kind:       canonicalKind(kind),
			namespace:  defaultNamespace(namespace),
			name:       name,
			containers: containerSetFromDoc(doc),
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

func containerSetFromDoc(doc map[string]any) map[string]bool {
	containers := map[string]bool{}
	spec, _ := doc["spec"].(map[string]any)
	template, _ := spec["template"].(map[string]any)
	templateSpec, _ := template["spec"].(map[string]any)
	rawContainers, _ := templateSpec["containers"].([]any)
	for _, raw := range rawContainers {
		container, _ := raw.(map[string]any)
		name, _ := container["name"].(string)
		if name != "" {
			containers[name] = true
		}
	}
	return containers
}

func containerSetsOverlap(a, b map[string]bool) bool {
	for name := range a {
		if b[name] {
			return true
		}
	}
	return false
}

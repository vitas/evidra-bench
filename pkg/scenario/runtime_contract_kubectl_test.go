package scenario

import (
	"strings"
)

func splitTypedName(value string) (string, string) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		return canonicalKind(value), ""
	}
	return canonicalKind(parts[0]), parts[1]
}

func valueAfterFlag(args []string, flag, fallback string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return fallback
}

func kubectlWaitTarget(args []string) string {
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if strings.Contains(arg, "/") {
			return arg
		}
	}
	return ""
}

func canonicalKind(kind string) string {
	switch strings.ToLower(kind) {
	case "deployment", "deployments":
		return "Deployment"
	case "service", "services":
		return "Service"
	case "application", "applications":
		return "Application"
	case "statefulset", "statefulsets":
		return "StatefulSet"
	case "crd", "customresourcedefinition", "customresourcedefinitions":
		return "CustomResourceDefinition"
	case "namespace", "namespaces":
		return "Namespace"
	case "pod", "pods":
		return "Pod"
	case "replicaset", "replicasets", "rs":
		return "ReplicaSet"
	case "persistentvolumeclaim", "persistentvolumeclaims", "pvc":
		return "PersistentVolumeClaim"
	case "persistentvolume", "persistentvolumes", "pv":
		return "PersistentVolume"
	case "daemonset", "daemonsets", "ds":
		return "DaemonSet"
	case "job", "jobs":
		return "Job"
	case "configmap", "configmaps", "cm":
		return "ConfigMap"
	case "secret", "secrets":
		return "Secret"
	default:
		return kind
	}
}

func defaultNamespace(namespace string) string {
	if namespace == "" {
		return "bench"
	}
	return namespace
}

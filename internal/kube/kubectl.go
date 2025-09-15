package kube

import (
    "bytes"
    "encoding/json"
    "fmt"
    "os/exec"
    "strings"
)

type CmdResult struct {
    Stdout string
    Stderr string
    Err    error
}

func baseArgs(context, kubeconfig string) []string {
    args := []string{}
    if context != "" {
        args = append(args, "--context", context)
    }
    if kubeconfig != "" {
        args = append(args, "--kubeconfig", kubeconfig)
    }
    return args
}

func Run(args []string, namespace, context, kubeconfig string, stdin []byte) (CmdResult, error) {
    full := append([]string{}, baseArgs(context, kubeconfig)...)
    if namespace != "" {
        full = append(full, "-n", namespace)
    }
    full = append(full, args...)
    cmd := exec.Command("kubectl", full...)
    if stdin != nil {
        cmd.Stdin = bytes.NewReader(stdin)
    }
    var out, errb bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &errb
    err := cmd.Run()
    res := CmdResult{Stdout: out.String(), Stderr: errb.String(), Err: err}
    if err != nil {
        return res, fmt.Errorf("kubectl %s: %v\n%s", strings.Join(args, " "), err, errb.String())
    }
    return res, nil
}

func ApplyYAML(doc string, namespace, context, kubeconfig string) (string, error) {
    res, err := Run([]string{"apply", "-f", "-"}, namespace, context, kubeconfig, []byte(doc))
    return res.Stdout, err
}

func DeleteYAML(doc string, namespace, context, kubeconfig string) (string, error) {
    res, err := Run([]string{"delete", "-f", "-", "--ignore-not-found=true"}, namespace, context, kubeconfig, []byte(doc))
    return res.Stdout, err
}

func GetJSON(resource, name, namespace, selector, context, kubeconfig string) (map[string]any, error) {
    args := []string{"get", resource}
    if name != "" { args = append(args, name) }
    if selector != "" { args = append(args, "-l", selector) }
    args = append(args, "-o", "json")
    res, err := Run(args, namespace, context, kubeconfig, nil)
    if err != nil { return nil, err }
    var data map[string]any
    if err := json.Unmarshal([]byte(res.Stdout), &data); err != nil {
        return nil, err
    }
    return data, nil
}

func GetServiceClusterIPs(namespace, context, kubeconfig, selector string) ([]string, error) {
    data, err := GetJSON("svc", "", namespace, selector, context, kubeconfig)
    if err != nil { return nil, err }
    items, _ := data["items"].([]any)
    ips := []string{}
    for _, it := range items {
        obj, _ := it.(map[string]any)
        spec, _ := obj["spec"].(map[string]any)
        ip, _ := spec["clusterIP"].(string)
        if ip != "" && ip != "None" { ips = append(ips, ip) }
    }
    return ips, nil
}

func GetDeploymentReplicas(name, namespace, context, kubeconfig string) (int, error) {
    data, err := GetJSON("deploy", name, namespace, "", context, kubeconfig)
    if err != nil { return 0, err }
    spec, _ := data["spec"].(map[string]any)
    if spec == nil { return 1, nil }
    if r, ok := spec["replicas"].(float64); ok { return int(r), nil }
    return 1, nil
}

func ScaleDeployment(name string, replicas int, namespace, context, kubeconfig string) (string, error) {
    res, err := Run([]string{"scale", "deploy", name, fmt.Sprintf("--replicas=%d", replicas)}, namespace, context, kubeconfig, nil)
    return res.Stdout, err
}

func DeleteByLabelKinds(namespace, context, kubeconfig string, kinds []string, selector string) (string, error) {
    // kubectl -n ns delete kind/kind -l selector
    args := []string{"delete"}
    for _, k := range kinds { args = append(args, k) }
    args = append(args, "-l", selector, "--ignore-not-found=true")
    res, err := Run(args, namespace, context, kubeconfig, nil)
    return res.Stdout, err
}

func DeleteClusterScopedByLabel(context, kubeconfig, resource, selector string) (string, error) {
    args := []string{"delete", resource, "-l", selector, "--ignore-not-found=true"}
    res, err := Run(args, "", context, kubeconfig, nil)
    return res.Stdout, err
}


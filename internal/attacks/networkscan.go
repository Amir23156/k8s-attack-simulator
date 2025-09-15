package attacks

import (
    "fmt"
    "time"

    "k8s-attack-simulator/internal/kube"
)

func JobName(prefix string) string {
    return fmt.Sprintf("%s-%d", prefix, time.Now().Unix())
}

func BuildNmapJobYAML(jobName string, targets []string, ports string, labels map[string]string) string {
    targetList := "kubernetes.default.svc.cluster.local"
    if len(targets) > 0 {
        // join with spaces as nmap accepts space-separated targets
        s := ""
        for i, t := range targets {
            if i > 0 { s += " " }
            s += t
        }
        targetList = s
    }
    cmd := fmt.Sprintf("nmap -Pn -sS -p %s %s || true", ports, targetList)
    labelsText := ""
    for k, v := range labels {
        labelsText += fmt.Sprintf("\n    %s: %s", k, v)
    }
    return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  labels:
    app: k8s-attack-simulator%s
spec:
  backoffLimit: 0
  template:
    metadata:
      labels:
        app: k8s-attack-simulator
    spec:
      restartPolicy: Never
      containers:
      - name: nmap
        image: instrumentisto/nmap:latest
        command: ["/bin/sh","-lc"]
        args: ["%s"]
`, jobName, labelsText, cmd)
}

func RunNetworkScan(namespace string, targets []string, ports, context, kubeconfig string) (string, error) {
    jobName := JobName("kas-net-scan")
    y := BuildNmapJobYAML(jobName, targets, ports, map[string]string{"app": "k8s-attack-simulator"})
    return kube.ApplyYAML(y, namespace, context, kubeconfig)
}


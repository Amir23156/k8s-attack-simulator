package simulator

import (
    "fmt"
    "strings"
    "time"

    "k8s-attack-simulator/internal/attacks"
    "k8s-attack-simulator/internal/kube"
)

const labelApp = "k8s-attack-simulator"

type Options struct {
    Namespace  string
    Context    string
    Kubeconfig string
    DryRun     bool
}

type Simulator struct { Opt Options }

func New(opt Options) *Simulator { return &Simulator{Opt: opt} }

// NetworkScan launches an in-cluster nmap Job against Service ClusterIPs.
func (s *Simulator) NetworkScan(ports, selector string, customTargets []string) (string, error) {
    targets := customTargets
    if len(targets) == 0 {
        var err error
        targets, err = kube.GetServiceClusterIPs(s.Opt.Namespace, s.Opt.Context, s.Opt.Kubeconfig, selector)
        if err != nil { return "", err }
    }
    if s.Opt.DryRun {
        y := attacks.BuildNmapJobYAML(attacks.JobName("kas-net-scan"), targets, ports, map[string]string{"app": labelApp})
        return fmt.Sprintf("[dry-run] Would apply Job with targets: %s\n%s\n", strings.Join(targets, ","), y), nil
    }
    return attacks.RunNetworkScan(s.Opt.Namespace, targets, ports, s.Opt.Context, s.Opt.Kubeconfig)
}

// ServiceDisruptionScale scales deployments, waits, then restores originals.
func (s *Simulator) ServiceDisruptionScale(deployments []string, replicas int, hold time.Duration) (string, error) {
    var out strings.Builder
    for _, d := range deployments {
        orig, err := kube.GetDeploymentReplicas(d, s.Opt.Namespace, s.Opt.Context, s.Opt.Kubeconfig)
        if err != nil { out.WriteString(fmt.Sprintf("get replicas %s: %v\n", d, err)); continue }
        if s.Opt.DryRun {
            out.WriteString(fmt.Sprintf("[dry-run] Would scale %s to %d, hold %s, then restore to %d.\n", d, replicas, hold.String(), orig))
            continue
        }
        if _, err := kube.ScaleDeployment(d, replicas, s.Opt.Namespace, s.Opt.Context, s.Opt.Kubeconfig); err != nil {
            out.WriteString(fmt.Sprintf("scale %s: %v\n", d, err))
            continue
        }
        time.Sleep(hold)
        if _, err := kube.ScaleDeployment(d, orig, s.Opt.Namespace, s.Opt.Context, s.Opt.Kubeconfig); err != nil {
            out.WriteString(fmt.Sprintf("restore %s: %v\n", d, err))
        } else {
            out.WriteString(fmt.Sprintf("disruption completed for %s (restored to %d)\n", d, orig))
        }
    }
    return out.String(), nil
}

// RBACPrivesc simulates a cluster-admin binding for a ServiceAccount; optional kubectl pod.
func (s *Simulator) RBACPrivesc(withPod bool) (string, error) {
    if s.Opt.DryRun {
        sa, crb, dep := attacks.BuildPrivescYAMLs("kas-privesc", labelApp, withPod)
        b := &strings.Builder{}
        b.WriteString("[dry-run] Would apply:\n")
        b.WriteString(sa); b.WriteString("\n---\n")
        b.WriteString(strings.ReplaceAll(crb, "PLACEHOLDER_NAMESPACE", s.Opt.Namespace))
        if withPod { b.WriteString("\n---\n"); b.WriteString(dep) }
        b.WriteString("\n")
        return b.String(), nil
    }
    return attacks.RunPrivesc(s.Opt.Namespace, s.Opt.Context, s.Opt.Kubeconfig, withPod)
}

// Cleanup deletes labeled namespaced resources and clusterrolebindings.
func (s *Simulator) Cleanup() (string, error) {
    var out strings.Builder
    if s.Opt.DryRun {
        out.WriteString(fmt.Sprintf("[dry-run] Would delete namespaced resources with label app=%s and clusterrolebindings with that label\n", labelApp))
        return out.String(), nil
    }
    if o, err := kube.DeleteByLabelKinds(s.Opt.Namespace, s.Opt.Context, s.Opt.Kubeconfig, []string{"job","deploy","pod","svc"}, "app="+labelApp); err != nil {
        out.WriteString(o)
        out.WriteString(fmt.Sprintf("cleanup ns error: %v\n", err))
    } else { out.WriteString(o) }

    if o, err := kube.DeleteClusterScopedByLabel(s.Opt.Context, s.Opt.Kubeconfig, "clusterrolebinding", "app="+labelApp); err != nil {
        out.WriteString(o)
        out.WriteString(fmt.Sprintf("cleanup crb error: %v\n", err))
    } else { out.WriteString(o) }
    return out.String(), nil
}

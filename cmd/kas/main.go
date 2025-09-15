package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "os"
    "strings"
    "time"

    "k8s-attack-simulator/simulator"
)

const labelApp = "k8s-attack-simulator"

type globalFlags struct {
    namespace  string
    context    string
    kubeconfig string
    dryRun     bool
}

func addGlobalFlags(fs *flag.FlagSet, gf *globalFlags) {
    fs.StringVar(&gf.namespace, "namespace", "default", "Target namespace")
    fs.StringVar(&gf.context, "context", "", "kubectl context (optional)")
    fs.StringVar(&gf.kubeconfig, "kubeconfig", "", "KUBECONFIG path (optional)")
    fs.BoolVar(&gf.dryRun, "dry-run", false, "Print intended actions without applying")
}

func usage() {
    fmt.Println("KAS - Kubernetes Attack & Anomaly Simulator")
    fmt.Println("Usage:")
    fmt.Println("  kas attack network-scan [flags]")
    fmt.Println("  kas attack service-disruption scale [flags]")
    fmt.Println("  kas attack rbac-privesc [flags]")
    fmt.Println("  kas cleanup [flags]")
}

func main() {
    if len(os.Args) < 2 {
        usage()
        os.Exit(1)
    }

    switch os.Args[1] {
    case "attack":
        attackCmd(os.Args[2:])
    case "cleanup":
        cleanupCmd(os.Args[2:])
    case "help", "--help", "-h":
        usage()
    default:
        fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
        usage()
        os.Exit(1)
    }
}

func attackCmd(args []string) {
    if len(args) < 1 {
        fmt.Fprintln(os.Stderr, "attack subcommand required")
        usage()
        os.Exit(1)
    }
    switch args[0] {
    case "network-scan":
        attackNetworkScan(args[1:])
    case "service-disruption":
        attackServiceDisruption(args[1:])
    case "rbac-privesc":
        attackRBACPrivesc(args[1:])
    default:
        fmt.Fprintf(os.Stderr, "unknown attack: %s\n", args[0])
        os.Exit(1)
    }
}

func attackNetworkScan(args []string) {
    fs := flag.NewFlagSet("network-scan", flag.ExitOnError)
    gf := &globalFlags{}
    addGlobalFlags(fs, gf)
    ports := fs.String("ports", "1-1024", "Port range or list for nmap")
    selector := fs.String("selector", "", "Label selector for Services (optional)")
    targetsCSV := fs.String("targets", "", "Comma-separated list of hosts/IPs to scan (overrides service discovery)")
    _ = fs.Parse(args)

    // parse targets
    var targets []string
    if t := strings.TrimSpace(*targetsCSV); t != "" {
        parts := strings.Split(t, ",")
        for _, p := range parts {
            s := strings.TrimSpace(p)
            if s != "" { targets = append(targets, s) }
        }
    }

    sim := simulator.New(simulator.Options{Namespace: gf.namespace, Context: gf.context, Kubeconfig: gf.kubeconfig, DryRun: gf.dryRun})
    out, err := sim.NetworkScan(*ports, *selector, targets)
    if err != nil {
        fmt.Fprintf(os.Stderr, "apply error: %v\n%s\n", err, out)
        os.Exit(1)
    }
    fmt.Print(out)
}

func attackServiceDisruption(args []string) {
    if len(args) < 1 {
        fmt.Fprintln(os.Stderr, "service-disruption subcommand required (scale)")
        os.Exit(1)
    }
    switch args[0] {
    case "scale":
        fs := flag.NewFlagSet("service-disruption scale", flag.ExitOnError)
        gf := &globalFlags{}
        addGlobalFlags(fs, gf)
        deploy := fs.String("deployment", "", "Deployment name (required unless --all)")
        replicas := fs.Int("replicas", 0, "Target replicas")
        duration := fs.Int("duration", 30, "Seconds to hold disruption before restore")
        all := fs.Bool("all", false, "Disrupt all profile deployments (profiles/gob.yaml)")
        profile := fs.String("profile", "profiles/gob.yaml", "Profile file for --all")
        _ = fs.Parse(args[1:])

        if !*all && *deploy == "" {
            fmt.Fprintln(os.Stderr, "--deployment required unless --all is set")
            os.Exit(1)
        }

        targets := []string{}
        if *all {
            names, err := readProfileDeployments(*profile)
            if err != nil { fmt.Fprintf(os.Stderr, "profile error: %v\n", err); os.Exit(1) }
            targets = names
        } else {
            targets = []string{*deploy}
        }
        sim := simulator.New(simulator.Options{Namespace: gf.namespace, Context: gf.context, Kubeconfig: gf.kubeconfig, DryRun: gf.dryRun})
        out, err := sim.ServiceDisruptionScale(targets, *replicas, time.Duration(*duration)*time.Second)
        if err != nil { fmt.Fprintf(os.Stderr, "disruption error: %v\n", err); os.Exit(1) }
        fmt.Print(out)
    default:
        fmt.Fprintf(os.Stderr, "unknown service-disruption subcommand: %s\n", args[0])
        os.Exit(1)
    }
}

func attackRBACPrivesc(args []string) {
    fs := flag.NewFlagSet("rbac-privesc", flag.ExitOnError)
    gf := &globalFlags{}
    addGlobalFlags(fs, gf)
    withPod := fs.Bool("with-pod", false, "Also deploy a kubectl pod bound to the escalated SA")
    _ = fs.Parse(args)

    sim := simulator.New(simulator.Options{Namespace: gf.namespace, Context: gf.context, Kubeconfig: gf.kubeconfig, DryRun: gf.dryRun})
    out, err := sim.RBACPrivesc(*withPod)
    if err != nil {
        fmt.Fprintf(os.Stderr, "privesc error: %v\n%s\n", err, out)
        os.Exit(1)
    }
    fmt.Print(out)
}

func cleanupCmd(args []string) {
    fs := flag.NewFlagSet("cleanup", flag.ExitOnError)
    gf := &globalFlags{}
    addGlobalFlags(fs, gf)
    _ = fs.Parse(args)

    sim := simulator.New(simulator.Options{Namespace: gf.namespace, Context: gf.context, Kubeconfig: gf.kubeconfig, DryRun: gf.dryRun})
    out, err := sim.Cleanup()
    if err != nil { fmt.Fprintf(os.Stderr, "cleanup error: %v\n", err); os.Exit(1) }
    fmt.Print(out)
}

func readProfileDeployments(path string) ([]string, error) {
    b, err := os.ReadFile(path)
    if err != nil { return nil, err }
    // minimal YAML-to-JSON via a naive approach: tolerate a simple list under key "deployments"
    // Example:
    // deployments:\n- frontend\n- cartservice
    type profile struct { Deployments []string `json:"deployments"` }
    // very small shim: convert a subset of YAML to JSON for this structure
    // We accept lines starting with "- " after a "deployments:" header.
    lines := strings.Split(string(b), "\n")
    var inList bool
    var items []string
    for _, ln := range lines {
        t := strings.TrimSpace(ln)
        if t == "deployments:" { inList = true; continue }
        if inList {
            if strings.HasPrefix(t, "- ") { items = append(items, strings.TrimSpace(strings.TrimPrefix(t, "- "))) } else if t != "" { inList = false }
        }
    }
    if len(items) == 0 { return nil, errors.New("no deployments found in profile") }
    // sanity JSON step unused but keeps type symmetry if needed
    _, _ = json.Marshal(profile{Deployments: items})
    return items, nil
}

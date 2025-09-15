Kubernetes Attack & Anomaly Simulator (KAS)

Overview

This project provides an attack and anomaly simulation framework for Kubernetes clusters. It enables security teams, SREs, and researchers to generate realistic events—such as system failures, misconfigurations, and security attacks—to test resilience, observability, and threat detection in cloud‑native systems.

The initial focus targets the Google Online Boutique (GoB) microservices demo, with controlled scenarios like privilege escalations, malicious network scans, and service disruptions. Future versions will expand to other workloads and integrate with AI/ML‑driven anomaly detection pipelines.

Status: MVP (Go CLI + kubectl orchestration)

Features (v0)

- Network scan: Launches an in‑cluster Job to scan Service IPs/Pods
- Service disruption: Temporarily scales deployments down, then restores
- RBAC privilege escalation: Creates a cluster‑admin binding for a ServiceAccount
- Dry‑run mode: Prints intended changes without applying
- Namespace scoping: Targets a chosen namespace safely
- Labels + cleanup: Resources labeled for easy cleanup

Requirements

- Go 1.20+
- kubectl available in PATH and configured (KUBECONFIG/context)
- Access to a Kubernetes cluster (user responsibility)

Quickstart

1) Explore commands

  - go run ./cmd/kas --help
  - go run ./cmd/kas attack --help

2) Target Google Online Boutique

  - Namespace: adjust as needed (e.g., online-boutique or default)
  - Profile file: profiles/gob.yaml lists common GoB deployments

3) Run a network scan against Services in namespace

  - go run ./cmd/kas attack network-scan --namespace online-boutique --ports 1-1024

4) Disrupt a service by scaling to zero for 60s (auto-restore)

  - go run ./cmd/kas attack service-disruption scale --namespace online-boutique --deployment frontend --replicas 0 --duration 60

5) Simulate RBAC privilege escalation (cluster-scope)

  - go run ./cmd/kas attack rbac-privesc --namespace online-boutique
  - Optionally launch a kubectl pod bound to the escalated SA:
    go run ./cmd/kas attack rbac-privesc --namespace online-boutique --with-pod

6) Cleanup all simulator-created resources in a namespace

  - go run ./cmd/kas cleanup --namespace online-boutique

Safety Notes

- Scenarios can be disruptive. Use on non‑production clusters unless you fully understand the impact.
- RBAC privilege escalation creates cluster‑scope bindings. Always run cleanup afterwards.
- Network scans may be noisy and trigger IDS/IPS; tune ports/targets accordingly.

Design

- CLI (Go, stdlib) shells out to kubectl
- All created resources carry label: app=k8s-attack-simulator
- Profiles provide workload‑specific defaults (GoB v0)

Profiles

- GoB profile: profiles/gob.yaml
  - Contains common deployment names for the Online Boutique demo
  - You can customize or provide your own profile files

Examples

- Network scan with dry-run:
  - go run ./cmd/kas attack network-scan --namespace online-boutique --ports 1-1024 --dry-run
  - Scan explicit targets (overrides service discovery):
    go run ./cmd/kas attack network-scan --namespace online-boutique --ports 22,80,443 --targets 10.0.0.10,frontend.online-boutique.svc.cluster.local

- Disrupt all GoB deployments by scaling to 0 for 30s:
  - go run ./cmd/kas attack service-disruption scale --namespace online-boutique --all --replicas 0 --duration 30

- RBAC privesc then open a shell in the attacker pod:
  - go run ./cmd/kas attack rbac-privesc --namespace online-boutique --with-pod
  - kubectl -n online-boutique exec -it deploy/kas-privesc-kubectl -- sh

Uninstall / Cleanup

- Namespaced resources: go run ./cmd/kas cleanup --namespace <ns>
- ClusterRoleBindings: included in cleanup; if something remains:
  - kubectl get clusterrolebinding -l app=k8s-attack-simulator
  - kubectl delete clusterrolebinding <name>

Roadmap

- Pod kill/evict chaos actions; network policies to blackhole traffic
- Data mutation anomalies; container filesystem tampering
- Scenario scheduling and replay; report/telemetry hooks
- Additional workload profiles; integration with anomaly detection pipelines

Tiny Resources (Minikube)

- Start a low-resource cluster (2 CPU, 1.5 GB):
  - minikube start --driver=docker --cpus=2 --memory=1536

- Deploy a tiny test app:
  - kubectl create ns kas-test || true
  - kubectl -n kas-test apply -f manifests/tiny-nginx.yaml
  - kubectl -n kas-test rollout status deploy/tiny-nginx --timeout=120s

- Run a quick scan and disruption:
  - go run ./cmd/kas attack network-scan --namespace kas-test --ports 80,22,443
  - go run ./cmd/kas attack service-disruption scale --namespace kas-test --deployment tiny-nginx --replicas 0 --duration 10

- Cleanup:
  - go run ./cmd/kas cleanup --namespace kas-test
  - kubectl delete ns kas-test

- Makefile shortcuts:
  - make minikube-start-tiny
  - make tiny-setup
  - make smoke-tiny
  - make tiny-clean

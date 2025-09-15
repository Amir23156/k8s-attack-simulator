APP=kas
CMD=./cmd/$(APP)
GOCACHE?=$(PWD)/.gocache
GOTMPDIR?=$(PWD)/.gotmp
GOFLAGS=
MINIKUBE_DRIVER?=docker
MINIKUBE_CPUS?=2
MINIKUBE_MEMORY?=1536

.PHONY: build run clean fmt vet

build:
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go build $(GOFLAGS) $(CMD)

run:
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go run $(GOFLAGS) $(CMD) $(ARGS)

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf $(GOCACHE) $(GOTMPDIR) $(APP)

.PHONY: minikube-start-tiny tiny-setup tiny-clean smoke-tiny

minikube-start-tiny:
	minikube start --driver=$(MINIKUBE_DRIVER) --cpus=$(MINIKUBE_CPUS) --memory=$(MINIKUBE_MEMORY)
	@echo "Context:" && kubectl config use-context minikube || true
	@kubectl get nodes

tiny-setup:
	@kubectl create ns kas-test 2>/dev/null || true
	@kubectl -n kas-test apply -f manifests/tiny-nginx.yaml
	@kubectl -n kas-test rollout status deploy/tiny-nginx --timeout=120s

tiny-clean:
	@GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go run $(GOFLAGS) $(CMD) cleanup --namespace kas-test || true
	@kubectl delete ns kas-test 2>/dev/null || true
	@kubectl delete clusterrolebinding -l app=k8s-attack-simulator --ignore-not-found=true || true

smoke-tiny: build tiny-setup
	@echo "[scan] running quick network scan"
	@GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go run $(GOFLAGS) $(CMD) attack network-scan --namespace kas-test --ports 80,22,443
	@echo "[disrupt] scaling tiny-nginx to 0 for 10s"
	@GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go run $(GOFLAGS) $(CMD) attack service-disruption scale --namespace kas-test --deployment tiny-nginx --replicas 0 --duration 10
	@echo "[cleanup] simulator cleanup"
	@GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go run $(GOFLAGS) $(CMD) cleanup --namespace kas-test
	@echo "[done] smoke test complete"

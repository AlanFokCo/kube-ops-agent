.PHONY: build test run clean docker-build docker-run helm-install helm-uninstall help

BINARY := k8sops
MAIN_PKG := ./cmd/k8sops
VERSION ?= $(shell grep -E '^\s*Version\s*=' version/version.go | awk -F'"' '{print $$2}')
DOCKER_IMAGE ?= kube-ops-agent
DOCKER_TAG ?= $(VERSION)

# Build
build:
	go build -o $(BINARY) $(MAIN_PKG)

# Test
test:
	go test -v ./...

# Run (set OPENAI_API_KEY first; default LLM self-planning)
run: build
	./$(BINARY)

# dry-run: list registered agents
dry-run: build
	./$(BINARY) --dry-run

# Run with Workflow (set OPENAI_API_KEY first)
run-workflow: build
	./$(BINARY) --workflow kubernetes-ops-agent/workflow.yaml

# Clean build artifacts
clean:
	rm -f $(BINARY)

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

# Run Docker container (mount kubeconfig and skills; default LLM self-planning)
docker-run:
	docker run --rm -it \
		-e OPENAI_API_KEY=$${OPENAI_API_KEY} \
		-v ~/.kube:/root/.kube:ro \
		-v $(PWD)/kubernetes-ops-agent/skills:/app/skills:ro \
		-v $(PWD)/kubernetes-ops-agent/report:/app/report \
		-p 8080:8080 \
		$(DOCKER_IMAGE):$(DOCKER_TAG) \
		--skills-dir /app/skills --report-dir /app/report --addr :8080

# Run Docker container with Workflow
docker-run-workflow:
	docker run --rm -it \
		-e OPENAI_API_KEY=$${OPENAI_API_KEY} \
		-v ~/.kube:/root/.kube:ro \
		-v $(PWD)/kubernetes-ops-agent/skills:/app/skills:ro \
		-v $(PWD)/kubernetes-ops-agent/workflow.yaml:/app/workflow.yaml:ro \
		-v $(PWD)/kubernetes-ops-agent/report:/app/report \
		-p 8080:8080 \
		$(DOCKER_IMAGE):$(DOCKER_TAG) \
		--skills-dir /app/skills --report-dir /app/report --workflow /app/workflow.yaml --addr :8080

# Helm install (set OPENAI_API_KEY first; default LLM self-planning)
helm-install:
	helm upgrade --install k8sops ./helm/kube-ops-agent \
		--namespace kube-ops-agent --create-namespace \
		--set image.repository=$(DOCKER_IMAGE) \
		--set image.tag=$(DOCKER_TAG) \
		--set openai.apiKey=$${OPENAI_API_KEY}

# Helm install with Workflow static orchestration
helm-install-workflow:
	helm upgrade --install k8sops ./helm/kube-ops-agent \
		--namespace kube-ops-agent --create-namespace \
		--set image.repository=$(DOCKER_IMAGE) \
		--set image.tag=$(DOCKER_TAG) \
		--set openai.apiKey=$${OPENAI_API_KEY} \
		--set workflow.enabled=true \
		--set workflow.configMap=default

# Helm uninstall
helm-uninstall:
	helm uninstall k8sops --namespace kube-ops-agent

# Help
help:
	@echo "Kube Ops Agent - Makefile targets"
	@echo ""
	@echo "  Build & Run"
	@echo "  build            Build binary"
	@echo "  test            Run tests"
	@echo "  run             Build and run (LLM self-planning)"
	@echo "  run-workflow    Run with Workflow static orchestration"
	@echo "  dry-run         Build and run in dry-run mode"
	@echo "  clean           Clean build artifacts"
	@echo "  deps            Download and tidy dependencies"
	@echo ""
	@echo "  Docker"
	@echo "  docker-build         Build Docker image"
	@echo "  docker-run          Run container (LLM self-planning)"
	@echo "  docker-run-workflow Run container (Workflow orchestration)"
	@echo ""
	@echo "  Helm"
	@echo "  helm-install         Install to K8s (LLM self-planning)"
	@echo "  helm-install-workflow Install with Workflow enabled"
	@echo "  helm-uninstall      Uninstall"
	@echo ""
	@echo "  help            Show this help"

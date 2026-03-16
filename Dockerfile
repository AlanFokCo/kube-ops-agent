# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o k8sops ./cmd/k8sops

# Runtime stage
FROM alpine:3.19

# Install kubectl for cluster inspection
ARG TARGETARCH
RUN apk add --no-cache ca-certificates curl && \
    curl -sLO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${TARGETARCH:-amd64}/kubectl" && \
    chmod +x kubectl && mv kubectl /usr/local/bin/ && \
    apk del curl

# Create non-root user
RUN adduser -D -u 1000 appuser

WORKDIR /app

# Copy binary from build stage
COPY --from=builder /build/k8sops /app/k8sops

# Create default data dirs (skills/report should be mounted via volume)
RUN mkdir -p /app/skills /app/report && chown -R appuser:appuser /app

USER appuser

# Default env (default LLM self-planning; set K8SOPS_WORKFLOW to enable Workflow)
ENV K8SOPS_HTTP_ADDR=:8080
ENV K8SOPS_SKILLS_DIR=/app/skills
ENV K8SOPS_REPORT_DIR=/app/report

EXPOSE 8080

ENTRYPOINT ["/app/k8sops"]
CMD ["--addr", ":8080", "--skills-dir", "/app/skills", "--report-dir", "/app/report"]

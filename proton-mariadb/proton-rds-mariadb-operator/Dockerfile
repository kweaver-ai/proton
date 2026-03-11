FROM quay.io/brancz/kube-rbac-proxy:v0.11.0 AS kube-rbac-proxy
FROM ubuntu:24.04 AS runtime
FROM golang:1.25 AS builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go


FROM runtime
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532
ENTRYPOINT ["/manager"]

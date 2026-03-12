FROM ubuntu:24.04 AS runtime
FROM golang:1.25 AS goenv

ENV CGO_ENABLED=0

WORKDIR /component-manage
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked,id=gomod \
    go mod download

##### tester stage
FROM goenv AS tester
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked,id=gomod \
    --mount=type=bind,target=/component-manage,readwrite \
    go tool golangci-lint run --out-format="colored-line-number:stdout,junit-xml:/ut-result/lint-results.xml" || true && \
    go tool gotestsum --packages="./..."  --junitfile ut-result/junit-results.xml  -- -gcflags="all=-N -l" --coverprofile=/ut-result/coverage.txt && \
    go tool gocov convert /ut-result/coverage.txt > /ut-result/coverage.json && \
    go tool gocov-xml < /ut-result/coverage.json > /ut-result/coverage.xml

##### builder stage
FROM goenv AS builder
ARG VERSION="unknown"
COPY config.yaml /app/config.yaml
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked,id=gomod \
    --mount=type=bind,target=/component-manage,readwrite \
    export LDFLAGS="$LDFLAGS -X main.date=$(date -u +%Y%m%d.%H%M%S)" && \
    export LDFLAGS="$LDFLAGS -X main.version=${VERSION}" && \
    export LDFLAGS="$LDFLAGS -X main.commitId=$(git rev-parse HEAD)" && \
    go build -ldflags="-s -w ${LDFLAGS}" -o /app/component-manage main.go

# 构建结果
FROM runtime AS build-result
ENV TZ=Asia/Shanghai LANG=en_US.UTF-8
COPY --from=builder /app /app
WORKDIR /app
CMD ["./component-manage", "serve"]

# 测试结果
FROM scratch AS test-result
COPY --from=tester /ut-result .
variable "VERSION" {
    default = "0.0.1-local"
}

variable "TAG" {
    default = "0.0.1-localtag"
}

variable "REPOSITORY_BASE" {
    default = "opensource"
}

variable "REGISTRY" {
    default = "acr.aishu.cn"
}

target "_platforms" {
    platforms = [
        "linux/amd64",
        "linux/arm64"
    ]
}

target "_label" {
    labels = {
        "org.opencontainers.image.source": "https://github.com/kweaver-ai/proton"
    }
}

target "cm-image" {
    inherits = ["_platforms", "_label"]
    target = "build-result"
    tags = [
        "${REGISTRY}/${REPOSITORY_BASE}/component-manage:${TAG}"
    ]
}

target "cm-chart" {
    dockerfile = "Dockerfile.chart"
    target = "result"
    output = [ ".buildx/charts" ]
    args = {
        VERSION = "${VERSION}"
        TAG = "${TAG}"
        REPOSITORY = "${REPOSITORY_BASE}/component-manage"
        REGISTRY = "${REGISTRY}"
    }
}

group "default" {
    targets = [
        "cm-image",
        "cm-chart"
    ]
}
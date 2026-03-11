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

target "rds-etcd" {
    context = "./proton-rds-etcd-image"
    tags = [
        "${REGISTRY}/${REPOSITORY_BASE}/rds-etcd:${TAG}"
    ]
}
target "rds-exporter" {
    context = "./proton-rds-exporter-image"
    tags = [
        "${REGISTRY}/${REPOSITORY_BASE}/rds-exporter:${TAG}"
    ]
}
target "rds-mariadb" {
    context = "./proton-rds-mariadb-image/src"
    tags = [
        "${REGISTRY}/${REPOSITORY_BASE}/rds-mariadb:${TAG}"
    ]
}
target "rds-mgmt" {
    context = "./proton-rds-mgmt-image"
    tags = [
        "${REGISTRY}/${REPOSITORY_BASE}/rds-mgmt:${TAG}"
    ]
}

target "rds-operator_image-controller" {
    context = "./proton-rds-mariadb-operator"
    tags = [
        "${REGISTRY}/${REPOSITORY_BASE}/rds-operator-controller:${TAG}"
    ]
}
target "rds-operator_image-proxy" {
    context = "./proton-rds-mariadb-operator"
    target = "kube-rbac-proxy"
    tags = [
        "${REGISTRY}/${REPOSITORY_BASE}/rds-operator-proxy:${TAG}"
    ]
}
target "rds-operator_chart" {
    context = "./proton-rds-mariadb-operator"
    dockerfile = "Dockerfile.chart"
    output = [ ".buildx/charts" ]
    args = {
        VERSION = "${VERSION}"
        TAG = "${TAG}"
        REPOSITORY_CONTROLLER = "${REPOSITORY_BASE}/rds-operator-controller"
        REPOSITORY_PROXY = "${REPOSITORY_BASE}/rds-operator-proxy"
        REGISTRY = "${REGISTRY}"
    }
}


group "rds-operator" {
    targets = [
        "rds-operator_image-controller",
        "rds-operator_image-proxy",
        "rds-operator_chart",
    ]
}

group "default" {
    targets = [
        "rds-etcd",
        "rds-exporter",
        "rds-mariadb",
        "rds-mgmt",
        "rds-operator"
    ]
}
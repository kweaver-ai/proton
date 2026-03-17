#!/usr/bin/bash

set -o errexit
set -o pipefail
set -o nounset

PKG_ROOT="$(dirname "${BASH_SOURCE[0]}")"

BIN_DIR="${PKG_ROOT}/bin"
REPO_DIR="${PKG_ROOT}/repos"

function install_binaries {
  local -a binaries
  readarray -t binaries < <(ls "${BIN_DIR}")
  for src in "${binaries[@]}"; do
    dst="/usr/local/bin/$(basename "${src}")"
    echo "Installing ${dst}"
    install "${BIN_DIR}/${src}" "${dst}"
  done
}

function install_rpm_packages {
  # 1. create rpm repository config
  sed "s|BASEURL|${REPO_DIR}|" < "${REPO_DIR}/proton.repo.tmpl" > "/etc/yum.repos.d/proton.repo"

  # 2. execute dnf install
  local names=(
    containerd
    ecms
    kubeadm
    kubectl
    kubelet
  )
  dnf install "${names[@]}" --repo=proton
}

install_binaries
install_rpm_packages

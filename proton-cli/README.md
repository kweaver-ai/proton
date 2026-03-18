# proton-cli

`proton-cli` is the Proton command-line tool for cluster deployment, configuration management, backup and recovery, Kubernetes maintenance, and offline package operations.

This README is written from the committed source code in this directory. It focuses on how to use the CLI, not on packaging artifacts that may exist outside the tracked code.

## Quick Start

### Requirements

- Linux environment
- Go 1.24 or newer to build from source
- A working Kubernetes client configuration at `~/.kube/config`
- For `apply`, a `service-package` directory that contains at least:
  - `charts/`
  - `images/`
- If you use `edit conf`, set `EDITOR`; otherwise `vi` is used

### Build

```bash
cd proton-cli
go build -o bin/proton-cli ./cmd/proton-cli
./bin/proton-cli --help
```

### Global Flags

Most commands accept these flags:

- `-l, --log-level`: log level, for example `info`, `debug`, `error`
- `-s, --service-package`: path to the `service-package` directory, default `service-package`
- `--service-package-eceph`: path to the `service-package-eceph` directory, default `service-package-eceph`
- `--cm-direct`: enable component-manage direct connection mode

## Common Workflows

### 1. Generate a cluster configuration template

Show one of the built-in templates:

```bash
proton-cli get template --type internal
proton-cli get template --type external
proton-cli get template --type perfrec
```

Create a file from a template:

```bash
proton-cli get template --type internal > cluster.yaml
```

Template types from the code:

- `internal`: deploy a local cluster
- `external`: deploy into an existing Kubernetes cluster
- `perfrec`: recommended configuration reference

### 2. Apply a cluster configuration

Apply a configuration file:

```bash
proton-cli apply -f cluster.yaml
```

Apply with an explicit deployment namespace:

```bash
proton-cli apply -f cluster.yaml -n proton
```

Apply with an explicit service package path:

```bash
proton-cli apply \
  -f cluster.yaml \
  -s /path/to/service-package \
  --service-package-eceph /path/to/service-package-eceph
```

Behavior confirmed from the code:

- `apply` reads the YAML file from `-f`
- `apply` loads package content from `service-package`
- if `-n` is given, it overrides the namespace from the config file
- if a namespace is chosen, `apply` updates the local file `~/.proton-cli.yaml`
- after a successful apply, the cluster configuration is uploaded into Kubernetes

### 3. Show the current stored cluster configuration

Read the current configuration from Kubernetes:

```bash
proton-cli get conf
```

Read from a specific namespace:

```bash
proton-cli get conf -n proton
```

`get conf` reads the configuration from the Kubernetes Secret `proton-cli-config`.

### 4. Edit the stored configuration in Kubernetes

Open the stored configuration in your editor:

```bash
proton-cli edit conf
```

Edit a specific namespace:

```bash
proton-cli edit conf -n proton
```

Important:

- this command edits the Secret content directly
- it uses `$EDITOR`, or `vi` if `$EDITOR` is unset
- the command itself prints: `ONLY change secrets, not apply!`

### 5. Backup and recovery

Create a backup:

```bash
proton-cli backup create --resources all
```

Create a backup with an explicit name and retention:

```bash
proton-cli backup create \
  --backupname nightly-001 \
  --resources all \
  --ttl 3
```

List backups and inspect logs:

```bash
proton-cli backup list
proton-cli backup log
proton-cli backup directory
proton-cli backup schedule
```

Create a recovery from a backup:

```bash
proton-cli recover create \
  --from-backup nightly-001 \
  --resources all
```

List recoveries and inspect logs:

```bash
proton-cli recover list
proton-cli recover log
```

### 6. Build and install an offline package

Print the built-in manifest template:

```bash
proton-cli offline-package plan > manifest.yaml
```

Build an offline package from a manifest:

```bash
proton-cli offline-package build --manifest manifest.yaml
```

The build command writes `proton-offline-package.tar` in the current directory.

Install an offline package:

```bash
proton-cli offline-package install proton-offline-package.tar
```

Keep the extracted working directory:

```bash
proton-cli offline-package install proton-offline-package.tar --remain
```

Behavior confirmed from the code:

- `build` reads `manifest.yaml` by default
- `plan` prints the embedded manifest template to stdout
- `install` extracts into `.proton-offline-package/` and runs `install.sh`

### 7. Kubernetes utilities

Show the current Kubernetes-related state:

```bash
proton-cli kubernetes show
```

Upgrade Calico:

```bash
proton-cli kubernetes calico upgrade <version>
```

The command validates the target version against a built-in supported version list before starting the upgrade.

### 8. Shell completion and version

Print version info:

```bash
proton-cli version
```

Enable shell completion:

```bash
proton-cli completion bash
proton-cli completion zsh
proton-cli completion fish
proton-cli completion powershell
```

## Command Overview

Root commands exposed by the committed code include:

- `apply`: apply a cluster configuration file
- `get`: show current configuration or built-in templates
- `edit`: edit stored configuration in Kubernetes
- `backup`: create and inspect backups
- `recover`: create and inspect recoveries
- `offline-package`: print a manifest template, build packages, install packages
- `kubernetes`: show Kubernetes state and manage Calico
- `completion`: generate shell completion
- `version`: print version information
- `precheck`: check node environment before install
- `reset`: reset a Proton cluster
- `migrate`: migrate components deployed by other programs
- `check`: check Proton runtime health
- `images`: manage images
- `push-images`: push images to a repository
- `push-charts`: push charts to a repository
- `package`: package-related commands
- `component`: data component management
- `delete-images`: delete images to reclaim disk space
- `server`: run the CLI server mode
- `alpha`: experimental commands

Use help on demand:

```bash
proton-cli --help
proton-cli <command> --help
```

## Runtime State and Default Locations

The committed code uses these default locations and names:

- Kubernetes client config: `~/.kube/config`
- local Proton CLI environment file: `~/.proton-cli.yaml`
- default Proton CLI config namespace: `proton`
- default Proton resource namespace: `resource`
- stored cluster configuration Secret: `proton-cli-config`
- cluster configuration key inside the Secret: `ClusterConfiguration`

In practice:

- `get conf` and `edit conf` read from `proton-cli-config`
- `apply` may update `~/.proton-cli.yaml`
- namespace-sensitive commands usually default to the namespace recorded in `~/.proton-cli.yaml`, or `proton` if the file does not exist

## Notes

- This README intentionally does not assume the repository already contains prebuilt `service-package` assets.
- If you are unsure about a command, prefer the built-in help output over older scripts or placeholders.

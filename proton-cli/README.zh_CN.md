# proton-cli

`proton-cli` 是 Proton 的命令行工具，用于集群部署、配置管理、备份恢复、Kubernetes 维护以及离线包操作。

本 README 基于当前目录中已经提交到仓库的源代码编写，重点说明 CLI 的使用方式，不假设仓库之外还存在其他未纳入版本控制的打包产物。

## 快速开始

### 环境要求

- Linux 环境
- 从源码构建时需要 Go 1.24 或更高版本
- 可用的 Kubernetes 客户端配置文件，路径为 `~/.kube/config`
- 使用 `apply` 时，需要一个至少包含以下内容的 `service-package` 目录：
  - `charts/`
  - `images/`
- 如果要使用 `edit conf`，请设置 `EDITOR`；否则默认使用 `vi`

### 构建

```bash
cd proton-cli
go build -o bin/proton-cli ./cmd/proton-cli
./bin/proton-cli --help
```

### 全局参数

大多数命令都支持以下参数：

- `-l, --log-level`：日志级别，例如 `info`、`debug`、`error`
- `-s, --service-package`：`service-package` 目录路径，默认值为 `service-package`
- `--service-package-eceph`：`service-package-eceph` 目录路径，默认值为 `service-package-eceph`
- `--cm-direct`：启用 component-manage 直连模式

## 常见工作流

### 1. 生成集群配置模板

显示内置模板之一：

```bash
proton-cli get template --type internal
proton-cli get template --type external
proton-cli get template --type perfrec
```

将模板输出到文件：

```bash
proton-cli get template --type internal > cluster.yaml
```

代码中内置的模板类型如下：

- `internal`：部署本地集群
- `external`：部署到已有 Kubernetes 集群
- `perfrec`：推荐配置参考

### 2. 应用集群配置

应用一个配置文件：

```bash
proton-cli apply -f cluster.yaml
```

显式指定部署命名空间：

```bash
proton-cli apply -f cluster.yaml -n proton
```

显式指定 service package 路径：

```bash
proton-cli apply \
  -f cluster.yaml \
  -s /path/to/service-package \
  --service-package-eceph /path/to/service-package-eceph
```

根据代码确认的行为：

- `apply` 会从 `-f` 读取 YAML 文件
- `apply` 会从 `service-package` 加载包内容
- 如果传入 `-n`，会覆盖配置文件中的命名空间
- 如果最终选择了某个命名空间，`apply` 会更新本地文件 `~/.proton-cli.yaml`
- `apply` 成功后，会把集群配置上传到 Kubernetes

### 3. 查看当前已保存的集群配置

从 Kubernetes 读取当前配置：

```bash
proton-cli get conf
```

从指定命名空间读取：

```bash
proton-cli get conf -n proton
```

`get conf` 会从 Kubernetes Secret `proton-cli-config` 中读取配置。

### 4. 编辑 Kubernetes 中保存的配置

在编辑器中打开当前保存的配置：

```bash
proton-cli edit conf
```

编辑指定命名空间中的配置：

```bash
proton-cli edit conf -n proton
```

注意：

- 该命令会直接修改 Secret 中保存的内容
- 它会使用 `$EDITOR`，如果未设置则使用 `vi`
- 命令本身会输出：`ONLY change secrets, not apply!`

### 5. 备份与恢复

创建备份：

```bash
proton-cli backup create --resources all
```

指定备份名称和保留数量：

```bash
proton-cli backup create \
  --backupname nightly-001 \
  --resources all \
  --ttl 3
```

查看备份列表和日志：

```bash
proton-cli backup list
proton-cli backup log
proton-cli backup directory
proton-cli backup schedule
```

基于备份创建恢复任务：

```bash
proton-cli recover create \
  --from-backup nightly-001 \
  --resources all
```

查看恢复列表和日志：

```bash
proton-cli recover list
proton-cli recover log
```

### 6. 构建并安装离线包

输出内置 manifest 模板：

```bash
proton-cli offline-package plan > manifest.yaml
```

基于 manifest 构建离线包：

```bash
proton-cli offline-package build --manifest manifest.yaml
```

构建命令会在当前目录生成 `proton-offline-package.tar`。

安装离线包：

```bash
proton-cli offline-package install proton-offline-package.tar
```

保留解压后的工作目录：

```bash
proton-cli offline-package install proton-offline-package.tar --remain
```

根据代码确认的行为：

- `build` 默认读取 `manifest.yaml`
- `plan` 会把内嵌的 manifest 模板输出到标准输出
- `install` 会先解压到 `.proton-offline-package/`，然后执行 `install.sh`

### 7. Kubernetes 工具

查看当前 Kubernetes 相关状态：

```bash
proton-cli kubernetes show
```

升级 Calico：

```bash
proton-cli kubernetes calico upgrade <version>
```

该命令会先根据内置支持版本列表校验目标版本，再开始升级。

### 8. Shell 补全与版本信息

输出版本信息：

```bash
proton-cli version
```

启用 shell 补全：

```bash
proton-cli completion bash
proton-cli completion zsh
proton-cli completion fish
proton-cli completion powershell
```

## 命令概览

当前已提交代码暴露的根命令包括：

- `apply`：应用集群配置文件
- `get`：查看当前配置或内置模板
- `edit`：编辑 Kubernetes 中保存的配置
- `backup`：创建并查看备份
- `recover`：创建并查看恢复任务
- `offline-package`：输出 manifest 模板、构建离线包、安装离线包
- `kubernetes`：查看 Kubernetes 状态并管理 Calico
- `completion`：生成 shell 补全脚本
- `version`：输出版本信息
- `precheck`：安装前检查节点环境
- `reset`：重置 Proton 集群
- `migrate`：迁移由其他程序部署的组件
- `check`：检查 Proton 运行时健康状态
- `images`：管理镜像
- `push-images`：把镜像推送到仓库
- `push-charts`：把 chart 推送到仓库
- `package`：与打包相关的命令
- `component`：数据组件管理
- `delete-images`：删除镜像以回收磁盘空间
- `server`：以 CLI Server 模式运行
- `alpha`：实验性命令

按需查看帮助：

```bash
proton-cli --help
proton-cli <command> --help
```

## 运行时状态与默认路径

当前已提交代码使用以下默认路径和名称：

- Kubernetes 客户端配置：`~/.kube/config`
- 本地 Proton CLI 环境文件：`~/.proton-cli.yaml`
- Proton CLI 默认配置命名空间：`proton`
- Proton 资源默认命名空间：`resource`
- 保存集群配置的 Secret：`proton-cli-config`
- Secret 中的集群配置字段名：`ClusterConfiguration`

实际行为上：

- `get conf` 和 `edit conf` 都会从 `proton-cli-config` 读取配置
- `apply` 可能会更新 `~/.proton-cli.yaml`
- 对命名空间敏感的命令通常会默认使用 `~/.proton-cli.yaml` 中记录的命名空间；如果该文件不存在，则默认使用 `proton`

## 说明

- 本 README 有意不假设仓库中一定已经包含可直接使用的 `service-package` 预构建产物。
- 如果你不确定某个命令怎么用，优先查看命令自身的帮助输出，而不是依赖旧脚本或占位文档。

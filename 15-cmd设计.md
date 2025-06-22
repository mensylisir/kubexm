### cmd模块设计
其位于pkg/cmd目录下
其结构如下
```aiignore
cmd/kubexm/
├── go.mod
├── main.go                     # main 函数入口 (注意: 路径应为 kubexm/main.go)
└── cmd/
    ├── root.go                 # 根命令 (kubexm)
    ├── version.go              # 'version' 命令,打印 CLI 和服务端的版本信息
    ├── completion.go           # 'completion' 命令,生成 shell 自动补全脚本 (bash, zsh, etc.)
    │
    ├── cluster/                # 集群管理
    │   ├── cluster.go          # 'cluster' 命令组
    │   ├── create.go           # 创建新集群
    │   ├── delete.go           # 删除集群
    │   ├── list.go             # 列出所有已创建的集群
    │   ├── get.go              # 获取特定集群的详细信息
    │   ├── upgrade.go          # 升级集群版本
    │   ├── add_nodes.go        # 为集群添加工作节点
    │   ├── delete_nodes.go     # 从集群删除工作节点
    │   ├── scale.go            # 调整集群节点数量或规格
    │   └── kubeconfig.go       # 获取集群的 kubeconfig 文件
    │
    ├── node/
    │   ├── node.go             # (新增) 'node' 命令组
    │   ├── list.go             # 列出集群中的所有节点
    │   ├── get.go              # 获取特定节点的详细信息
    │   ├── cordon.go           # 标记节点为不可调度
    │   └── drain.go            # 安全地驱逐节点上的 Pods 并标记为不可调度
    │
    ├── certs/
    │   ├── certs.go            # 'certs' 命令组
    │   ├── update.go           # 更新/轮换证书
    │   ├── check_expiration.go # 检查证书的过期时间
    │   └── rotate.go           # 轮换特定的服务证书
    │
    └── config/
        ├── config.go           # (新增) 'config' 命令组
        ├── set.go              # 设置 CLI 配置项 (如 API key, 默认区域)
        ├── view.go             # 查看当前的 CLI 配置
        └── use_context.go      # 切换不同的环境/集群上下文
```
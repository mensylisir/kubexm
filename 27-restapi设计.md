您设计的 cmd 模块结构清晰，资源和动作分离得很好，这为设计一个遵循 RESTful 风格的 API 提供了绝佳的蓝图。

下面我将根据您的 cmd 模块设计，为您设计一套对应的 REST API。

### API 设计核心原则

1. **资源导向 (Resource-Oriented)**: API 的核心是资源。URL应该代表资源（名词），而不是动作（动词）。
    - cluster -> /clusters
    - node -> /clusters/{cluster_id}/nodes (节点是集群的子资源)
    - certs -> /clusters/{cluster_id}/certificates (证书也是集群的子资源)
2. **标准 HTTP 方法**: 使用标准的 HTTP 动词来表示对资源的操作。
    - GET: 读取资源（单个或列表）。
    - POST: 创建新资源，或对资源执行一个动作（Action）。
    - DELETE: 删除资源。
    - PATCH / PUT: 更新资源。PATCH 用于部分更新，PUT 用于完整替换。
3. **版本化 (Versioning)**: API 应包含版本号，以确保向后兼容。例如 /api/v1/...。
4. **异步处理 (Asynchronous Operations)**: 像创建集群、升级这样耗时的操作应该是异步的。API 应立即返回一个 202 Accepted 状态码，并提供一个任务/操作 ID，客户端可以通过该 ID 查询操作状态。
5. **清晰的响应**: 使用标准的 HTTP 状态码，并提供结构化的 JSON 响应体，包括错误信息。

------



### REST API 设计详情

#### API Base Path: /api/v1

------



### 1. 集群资源 (Cluster Resources)

**Endpoint Base**: /api/v1/clusters

















| 动作 (Action)       | CLI 命令           | HTTP 方法 | Endpoint                                 | 描述与响应                                                   |
| ------------------- | ------------------ | --------- | ---------------------------------------- | ------------------------------------------------------------ |
| **创建集群**        | cluster create     | POST      | /api/v1/clusters                         | **Request Body**: 包含集群配置的 JSON (等同于 cluster.yaml)。<br>**Success Response**: 202 Accepted。返回一个操作对象，包含 operationId 用于追踪进度。 |
| **列出集群**        | cluster list       | GET       | /api/v1/clusters                         | **Success Response**: 200 OK。返回一个包含所有集群摘要信息的 JSON 数组。支持分页 (?limit=10&offset=0)。 |
| **获取集群详情**    | cluster get        | GET       | /api/v1/clusters/{cluster_id}            | **Success Response**: 200 OK。返回指定集群的完整详细信息。   |
| **删除集群**        | cluster delete     | DELETE    | /api/v1/clusters/{cluster_id}            | **Success Response**: 202 Accepted。返回一个操作对象，包含 operationId。 |
| **获取 KubeConfig** | cluster kubeconfig | GET       | /api/v1/clusters/{cluster_id}/kubeconfig | **Success Response**: 200 OK。响应体直接是 kubeconfig 文件内容 (通常是 text/plain 或 application/yaml)。 |
| **对集群执行动作**  | (多个命令)         | POST      | /api/v1/clusters/{cluster_id}/actions    | 这是一个用于触发非 CRUD 操作的通用端点。<br>**Request Body**: {"action": "action_name", "params": {...}}<br><br>**upgrade**: { "action": "upgrade", "params": { "version": "1.25.4" } }<br>**add_nodes**: { "action": "add_nodes", "params": { "nodes": [...] } }<br>**delete_nodes**: { "action": "delete_nodes", "params": { "node_ids": [...] } }<br>**scale**: { "action": "scale", "params": { "worker_count": 5 } }<br><br>**Success Response**: 202 Accepted。返回操作 ID。 |

------



### 2. 节点资源 (Node Resources)

**Endpoint Base**: /api/v1/clusters/{cluster_id}/nodes











| 动作 (Action)      | CLI 命令           | HTTP 方法 | Endpoint                                              | 描述与响应                                                   |
| ------------------ | ------------------ | --------- | ----------------------------------------------------- | ------------------------------------------------------------ |
| **列出节点**       | node list          | GET       | /api/v1/clusters/{cluster_id}/nodes                   | **Success Response**: 200 OK。返回指定集群下所有节点的摘要信息数组。 |
| **获取节点详情**   | node get           | GET       | /api/v1/clusters/{cluster_id}/nodes/{node_id}         | **Success Response**: 200 OK。返回指定节点的完整详细信息。   |
| **对节点执行动作** | node cordon, drain | POST      | /api/v1/clusters/{cluster_id}/nodes/{node_id}/actions | 类似集群的动作端点。<br>**Request Body**: {"action": "action_name", "params": {...}}<br><br>**cordon**: { "action": "cordon" }<br>**drain**: { "action": "drain", "params": { "force": true, "ignore_daemonsets": true } }<br><br>**Success Response**: 202 Accepted。返回操作 ID。 |

------



### 3. 证书资源 (Certificate Resources)

**Endpoint Base**: /api/v1/clusters/{cluster_id}/certificates









| 动作 (Action)    | CLI 命令               | HTTP 方法 | Endpoint                                                     | 描述与响应                                                   |
| ---------------- | ---------------------- | --------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| **检查证书过期** | certs check_expiration | GET       | /api/v1/clusters/{cluster_id}/certificates/expiration-status | **Success Response**: 200 OK。返回一个 JSON 对象，包含每个证书的名称、颁发者和过期时间。 |
| **轮换证书**     | certs rotate, update   | POST      | /api/v1/clusters/{cluster_id}/certificates/actions           | **Request Body**: { "action": "rotate", "params": { "services": ["etcd", "kubelet"] } } (如果 services 为空，则轮换所有)。<br>**Success Response**: 202 Accepted。返回操作 ID。 |

------



### 4. 系统级与辅助端点









| 功能             | CLI 命令     | HTTP 方法 | Endpoint                          | 描述与响应                                                   |
| ---------------- | ------------ | --------- | --------------------------------- | ------------------------------------------------------------ |
| **获取版本信息** | version      | GET       | /version                          | **Success Response**: 200 OK。返回一个包含 API 版本、Git Commit 等信息的 JSON 对象。 |
| **追踪操作状态** | (无直接命令) | GET       | /api/v1/operations/{operation_id} | 用于轮询异步操作的状态。<br>**Success Response**: 200 OK。返回操作对象，包含 status ("pending", "running", "succeeded", "failed")、progress、error_message 等字段。 |

------



### 不适用于 REST API 的 cmd 命令

以下 cmd 命令是纯粹的客户端工具，不适合转换为服务器端的 REST API：

- **cmd/completion/**: Shell 自动补全是 CLI 的特性，与后端 API 无关。
- **cmd/config/**: 这组命令 (set, view, use_context) 用于管理 **本地 CLI 的配置文件**（例如 ~/.kubexm/config），用于存储 API 地址、认证凭据等。API 本身是无状态的，它假设客户端已经通过某种方式（如 Authorization: Bearer <token> HTTP 头）完成了认证。

### 总结

这个 REST API 设计方案紧密地映射了您现有的 CLI 功能，同时遵循了现代 API 设计的最佳实践。它为您的 kubexm 工具提供了一个强大、可预测且易于集成的编程接口。
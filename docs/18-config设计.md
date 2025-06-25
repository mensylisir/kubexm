### pkg/config: 加载并解析 config。
#### 它的 ParseFromFile 函数将 cluster.yaml 文件转换为内存中的 *apis.v1alpha1.Cluster 对象


-

### 整体评价：从“文件”到“内存对象”的可靠翻译官

**职责与设计思想**:

- **核心职责**: 负责将用户提供的YAML配置文件（如cluster.yaml）安全、准确地解析并反序列化为pkg/apis/kubexms/v1alpha1.Cluster的Go结构体对象。
- **设计目标**:
    1. **可靠性**: 必须能正确处理各种合法的YAML语法。
    2. **验证**: 在解析后，应执行必要的结构性验证和默认值填充。
    3. **解耦**: config模块只关心“加载和解析”，不应包含任何与执行、连接相关的业务逻辑。它是一个纯粹的数据转换层。
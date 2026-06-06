# Design: SD-03分布式缓存系统

## 架构概览

SD-03分布式缓存系统采用经典的**客户端-服务器**架构，核心链路为：
**客户端** → **自定义二进制协议编解码** → **TCP服务器** → **一致性哈希分片路由** → **多缓存节点（LRU缓存模块）** → **主从复制同步**

整体数据流向如下：

1. **客户端请求**：客户端通过TCP连接发送二进制协议帧
2. **协议解析**：TCP服务器接收网络数据，进行帧解析和校验
3. **命令分发**：根据命令类型（GET/SET/DELETE/INFO）路由到对应处理器
4. **分片定位**：通过一致性哈希环确定请求所属的缓存分片节点
5. **缓存操作**：在分片节点的LRU缓存模块执行具体缓存操作
6. **结果返回**：将响应数据序列化并返回给客户端
7. **主从同步**：主节点的写操作同步到从节点，保证数据一致性

系统支持水平扩展，通过一致性哈希分片将数据分布在多个节点上，同时通过简化版主从复制实现高可用性。

### 系统架构图

```text
+----------------+      +-------------------+      +-------------------+
|   Client       | ---> | Protocol Codec    | ---> | TCP Server        |
+----------------+      +-------------------+      +-------------------+
                                                        |
                                        +--------------v--------------+
                                        |   Consistent Hash Ring    |
                                        +--------------+--------------+
                                                     |
                                     +--------------v--------------+
                                     |   Hash Ring Routing        |
                                     +--------------+--------------+
                                                     |
                     +------------------------------v------------------------------+
                     |                       Cache Cluster                        |
                     | +-------------------+  +-------------------+  +-------------------+ |
                     | |   Cache Node 1    |  |   Cache Node 2    |  |   Cache Node 3    | |
                     | | (Master/Slave)    |  | (Master/Slave)    |  | (Master/Slave)    | |
                     | +----------+--------+  +----------+--------+  +----------+--------+ |
                     |            |                       |                       |        |
                     |  LRU       |                       |                       |  LRU    |
                     | Cache      |                       |                       | Cache   |
                     |            v                       v                       v        |
                     |     +------v------+         +------v------+         +------v------+ |
                     |     | Item Node 1 |         | Item Node 1 |         | Item Node 1 | |
                     |     | Item Node 2 |         | Item Node 2 |         | Item Node 2 | |
                     |     | Item Node 3 |         | Item Node 3 |         | Item Node 3 | |
                     |     +------------+         +------------+         +------------+ |
                     +-----------------------------+-----------------------------+
                                                           |
                                                           | Replication Sync
                                                           v
                                                    +-------------------+
                                                    | Slave Nodes       |
                                                    +-------------------+
```

**架构说明**：

1. **协议层（Protocol Codec）**：
   - 将客户端请求序列化为二进制协议帧
   - 将服务器响应反序列化为客户端可读格式
   - 帧格式：Command (1B) + KeyLen (4B) + ValueLen (4B) + Data
   - 使用大端字节序（Big-Endian，网络字节序）

2. **网络层（TCP Server）**：
   - 监听端口7000，支持多客户端并发连接
   - 每个连接独立处理请求-响应
   - 异常处理和资源清理

3. **路由层（Consistent Hash Ring）**：
   - 基于一致性哈希算法将Key映射到节点
   - 虚拟节点机制（默认每个物理节点100个虚拟节点）
   - 负载均衡和自动数据重分配

4. **数据层（Cache Nodes）**：
   - 每个节点维护独立的LRU缓存实例
   - LRU算法基于双向链表+哈希表实现
   - 支持节点状态管理（Master/Slave）

5. **复制层（Replication）**：
   - 主节点写操作同步到从节点
   - 原主节点故障后从节点提升
   - 原主节点重启后同步新数据

## 模块划分

### Module 1：协议编解码模块（protocol）

**职责**：
- 定义和实现自定义二进制协议格式
- 负责请求/响应数据的序列化和反序列化
- 实现协议校验（帧长度、校验码等）
- 定义命令码和错误码

**接口**：
```go
// 定义协议常量
const (
    // 命令码
    CMD_GET   = 0x01
    CMD_SET   = 0x02
    CMD_DELETE = 0x03
    CMD_INFO  = 0x04

    // 错误码
    SUCCESS              = 0x00
    ERROR_UNKNOWN_COMMAND = 0x01
    ERROR_INVALID_KEY    = 0x02
    ERROR_INVALID_VALUE  = 0x03
    ERROR_CACHE_FULL     = 0x04
)

// 协议帧结构
type ProtocolFrame struct {
    Command uint8     // 命令码
    KeyLen  uint32    // Key长度（4字节）
    ValueLen uint32   // Value长度（4字节）
    Key     []byte    // Key数据
    Value   []byte    // Value数据
    // 校验码可后续扩展
}

// 序列化请求
// 返回error: 参数为空、长度超限、序列化失败等错误
func EncodeRequest(cmd uint8, key, value []byte) ([]byte, error)

// 反序列化请求
// 返回error: 数据不足、校验失败、解析错误等错误
func DecodeRequest(data []byte) (*ProtocolFrame, error)

// 序列化响应
// 返回error: 序列化失败等错误
func EncodeResponse(cmd uint8, status uint8, value []byte) ([]byte, error)

// 反序列化响应
// 返回error: 数据不足、校验失败、解析错误等错误
func DecodeResponse(data []byte) (*ProtocolFrame, error)

// 验证协议帧有效性
// 返回error: 帧长度不足、Key/Value长度不匹配等错误
func ValidateFrame(frame *ProtocolFrame) error
```

**设计要点**：
- 帧格式：Command (1B) + KeyLen (4B) + ValueLen (4B) + Data (Var)
- 命令码和错误码已明确定义在proposal中
- 支持GET/SET/DELETE/INFO命令
- 对非法命令返回ERROR_UNKNOWN_COMMAND

---

### Module 2：LRU缓存模块（cache）

**职责**：
- 实现标准的LRU（Least Recently Used）缓存淘汰算法
- 支持GET/SET/DELETE基本缓存操作
- 维护缓存容量限制，达到上限时自动淘汰最少使用的条目
- 使用**container/list标准库**实现双向链表+哈希表，实现O(1)时间复杂度的操作

**接口**：
```go
// LRU缓存结构
type LRUCache struct {
    capacity int                        // 缓存容量
    list     *list.List                 // container/list双向链表
    cache    map[string]*list.Element   // 哈希表，O(1)查找
    mu       sync.Mutex                 // 互斥锁保护并发访问
}

// 初始化LRU缓存
// 返回error: capacity <= 0等参数错误
func NewLRUCache(capacity int) (*LRUCache, error)

// GET操作：获取指定Key的值，若存在则移到链表头部
// 返回: value, found - 若found为false表示Key不存在
func (c *LRUCache) Get(key string) ([]byte, bool)

// SET操作：设置Key-Value，若超过容量则淘汰最少使用的条目
// 返回error: 缓存已满且无法淘汰时返回ERROR_CACHE_FULL
func (c *LRUCache) Set(key string, value []byte) error

// DELETE操作：删除指定Key
// 返回bool: 是否删除成功
func (c *LRUCache) Delete(key string) bool

// 获取当前缓存大小
// 返回int: 当前缓存中的条目数
func (c *LRUCache) Size() int

// 获取缓存是否已满
// 返回bool: 若当前大小等于capacity则返回true
func (c *LRUCache) IsFull() bool

// 清空缓存
// 返回error: 无异常情况
func (c *LRUCache) Clear() error
```

**设计要点**：
- **使用自定义双向链表实现**（不依赖container/list）：
  - 优势：直接展示LRU算法实现原理，便于理解和学习
  - 减少代码复杂度，更易于调试和维护
  - 缓存满时，从链表尾部移除最少使用的条目（O(1)操作）
  - GET/SET操作成功后，将条目移动到链表头部（最近使用）
  - DELETE操作后，从哈希表和链表中移除条目
- 哈希表存储CacheItem指针，O(1)时间复杂度查找

---

### Module 3：一致性哈希分片模块（shard）

**职责**：
- 实现一致性哈希（Consistent Hashing）算法
- 通过虚拟节点机制实现数据分片和负载均衡
- 负责将Key映射到具体的缓存节点
- 支持动态添加/移除节点时的数据重分配

**接口**：
```go
// 哈希环节点结构
type VirtualNode struct {
    NodeID      string         // 物理节点ID
    VirtualNode string         // 虚拟节点名称（NodeID#1, NodeID#2...）
    Hash        uint64         // 哈希值
    Weight      int            // 节点权重（可选）
}

// 哈希环结构
type HashRing struct {
    virtualNodes int            // 每个物理节点的虚拟节点数（默认100）
    nodeCount    int            // 当前物理节点数量
    sortedNodes  []uint64       // 已排序的哈希值数组
    ring         []VirtualNode  // 虚拟节点数组
    mu           sync.RWMutex   // 读写锁保护并发访问
}

// 初始化哈希环
// 返回error: 虚拟节点数<=0等参数错误
func NewHashRing(virtualNodes int) (*HashRing, error)

// 添加物理节点到哈希环
// 返回error: 节点ID为空等参数错误
func (r *HashRing) AddNode(nodeID string) error

// 移除物理节点从哈希环
// 返回error: 节点ID为空等参数错误
func (r *HashRing) RemoveNode(nodeID string) error

// 根据Key确定所属的节点
// 返回string: 节点ID，若哈希环为空则返回空字符串
func (r *HashRing) GetNode(key string) string

// 获取当前节点列表
// 返回[]string: 所有节点的ID列表
func (r *HashRing) GetNodes() []string

// 重新构建哈希环（添加/移除节点后调用）
// 返回error: 构建失败时的错误
func (r *HashRing) Rebuild() error
```

**设计要点**：
- 使用标准库的`hash/fnv`算法计算哈希值（FNV-1a）
- 每个物理节点默认创建100个虚拟节点
- 虚拟节点均匀分布在哈希环上，保证负载均衡
- 添加新节点时，约10-20%的数据需要迁移（基于环形区间）
- 移除节点时，该节点数据由环中后继节点接管

---

### Module 4：缓存节点模块（node）

**职责**：
- 管理单个缓存节点的LRU缓存实例
- 与哈希环集成，提供统一的缓存访问接口
- 处理来自分片的缓存操作请求
- 支持节点的启动、停止和状态管理

**接口**：
```go
// 缓存节点结构
type CacheNode struct {
    ID          string            // 节点ID
    capacity    int               // 缓存容量
    lru         *LRUCache         // LRU缓存实例
    ring        *HashRing         // 所属的哈希环
    status      string            // 节点状态（"Running", "Master", "Slave"）
    masterID    string            // 主节点ID（仅Slave有值）
    mu          sync.RWMutex      // 读写锁
}

// 创建缓存节点
// 返回error: id为空或capacity <= 0等参数错误
func NewCacheNode(id string, capacity int) (*CacheNode, error)

// 初始化节点（初始化哈希环、LRU缓存）
// 返回error: 初始化失败时的错误
func (n *CacheNode) Init(ring *HashRing) error

// 获取缓存值
// 返回error: 若节点未初始化或LRU未初始化
func (n *CacheNode) Get(key string) ([]byte, error)

// 设置缓存值
// 返回error: 若节点未初始化或LRU未初始化
func (n *CacheNode) Set(key string, value []byte) error

// 删除缓存值
// 返回error: 若节点未初始化或LRU未初始化
func (n *CacheNode) Delete(key string) error

// 获取节点信息
// 返回map[string]interface{}: 节点的详细信息
func (n *CacheNode) GetInfo() map[string]interface{}

// 获取节点ID
// 返回string: 节点的ID
func (n *CacheNode) ID() string

// 获取缓存大小
// 返回int: 当前缓存中的条目数
func (n *CacheNode) Size() int

// 启动节点
// 返回error: 启动失败时的错误（如端口占用等）
func (n *CacheNode) Start() error

// 停止节点
// 返回error: 停止失败时的错误
func (n *CacheNode) Stop() error

// 设置节点状态
// 返回error: 状态值无效时的错误
func (n *CacheNode) SetStatus(status string) error
```

**设计要点**：
- 缓存节点作为LRU缓存和哈希环的容器
- 提供统一的GET/SET/DELETE接口
- 支持节点状态管理（运行中、Master、Slave）
- 线程安全，使用读写锁保护共享数据

---

### Module 5：TCP服务器模块（server）

**职责**：
- 提供基于TCP的缓存服务器，支持多客户端并发连接
- 接收网络数据，进行协议解析和命令分发
- 管理客户端连接的生命周期
- 处理网络异常和客户端断开连接

**接口**：
```go
// TCP服务器结构
type TCPServer struct {
    address     string            // 监听地址（默认":7000"）
    listener    net.Listener      // TCP监听器
    nodes       []*CacheNode      // 缓存节点列表
    mu          sync.RWMutex      // 读写锁
    stopChan    chan struct{}     // 停止信号通道
}

// 创建TCP服务器
// 返回error: address无效或nodes为空等参数错误
func NewTCPServer(address string, nodes []*CacheNode) (*TCPServer, error)

// 启动服务器
// 返回error: 监听端口失败、地址已占用等错误
func (s *TCPServer) Start() error

// 停止服务器
// 返回error: 停止失败时的错误
func (s *TCPServer) Stop() error

// 处理客户端连接
func (s *TCPServer) handleConnection(conn net.Conn)

// 处理客户端请求
// 返回error: 协议解析错误、命令处理错误等
func (s *TCPServer) handleRequest(conn net.Conn, frame *ProtocolFrame) error

// 命令分发器
// 返回error: 命令处理失败时的错误
func (s *TCPServer) dispatchCommand(conn net.Conn, cmd uint8, key, value []byte) (*ProtocolFrame, error)

// 处理GET命令
// 返回error: 哈希环为空或查询失败等错误
func (s *TCPServer) handleGet(key string) (*ProtocolFrame, error)

// 处理SET命令
// 返回error: 哈希环为空或写入失败等错误
func (s *TCPServer) handleSet(key string, value []byte) (*ProtocolFrame, error)

// 处理DELETE命令
// 返回error: 哈希环为空或删除失败等错误
func (s *TCPServer) handleDelete(key string) (*ProtocolFrame, error)

// 处理INFO命令
// 返回error: 获取节点信息失败等错误
func (s *TCPServer) handleInfo() (*ProtocolFrame, error)
```

**设计要点**：
- 使用Go标准库的`net`包实现TCP服务器
- 支持多客户端并发连接（通过goroutine处理每个连接）
- 每个连接独立处理请求-响应
- 支持优雅关闭（通过stopChan通道）
- 网络异常和客户端断开需正确处理和资源清理

---

### Module 6：主从复制模块（replication）

**职责**：
- 实现简化版的主从复制机制
- 主节点的写操作同步到从节点
- 从节点断开重连后请求全量数据同步
- 支持主节点故障后从节点提升为新主节点
- 原主节点重启后连接到新Master并同步数据

**接口**：
```go
// 复制协议数据结构
type ReplicationCommand struct {
    Command uint8         // 复制命令类型
    Key     string        // Key
    Value   []byte        // Value
    Count   uint32        // 数据条数（全量同步时使用）
}

// 复制状态
type ReplicationState struct {
    MasterID     string            // 当前主节点ID
    SlaveID      string            // 从节点ID
    IsMaster     bool              // 是否为主节点
    SyncedCount  int               // 已同步数据条数
    LastSyncTime time.Time         // 最后同步时间
    mu           sync.Mutex
}

// 主从复制控制器
type ReplicationController struct {
    nodes []*CacheNode            // 所有缓存节点
    state  *ReplicationState      // 复制状态
    mu     sync.Mutex
}

// 创建复制控制器
// 返回error: nodes为空等参数错误
func NewReplicationController(nodes []*CacheNode) (*ReplicationController, error)

// 主节点执行SET操作后同步到从节点
// 返回error: 节点未初始化、同步失败等错误
func (rc *ReplicationController) SyncToSlave(key string, value []byte) error

// 从节点初始化同步（连接主节点后调用）
// 返回error: masterID为空、初始化失败等错误
func (rc *ReplicationController) InitSync(masterID string) error

// 从节点请求全量数据同步
// 返回error: masterID为空、请求失败等错误
func (rc *ReplicationController) RequestFullSync(masterID string) ([]ReplicationCommand, error)

// 从节点接收全量数据并应用
// 返回error: commands为空、应用失败等错误
func (rc *ReplicationController) ApplyFullSync(commands []ReplicationCommand) error

// 主节点故障检测
// 返回(string, error): 失败的主节点ID或错误
func (rc *ReplicationController) DetectMasterFailure() (string, error)

// 从节点提升为新主节点
// 返回error: 从节点未初始化、提升失败等错误
func (rc *ReplicationController) PromoteSlave(slaveID string) error

// 设置节点主从关系
// 返回error: masterID或slaveID为空、设置失败等错误
func (rc *ReplicationController) SetMasterSlave(masterID, slaveID string) error
```

**设计要点**：
- 简化版复制：主节点写操作同步到从节点（同步复制）
- 从节点断开重连后，请求全量同步所有数据
- 主节点故障后，从节点提升为新主节点（无选举算法，简化实现）
- 原主节点重启后，连接到新Master并同步数据
- 响应延迟目标：< 10ms（10ms内同步到从节点）

---

## 数据模型

### 1. 协议帧数据模型
```
ProtocolFrame
├── Command (uint8): 命令码（0x01=GET, 0x02=SET, 0x03=DELETE, 0x04=INFO）
├── KeyLen (uint32): Key长度（大端字节序）
├── ValueLen (uint32): Value长度（大端字节序）
├── Key ([]byte): Key数据
└── Value ([]byte): Value数据
```

**帧格式示例（GET命令）**：
```
+----------------+----------------+---------------+--------------------+
| 0x01 (GET)     | 0x00000005     | 0x00000000    | "Key1"             |
+----------------+----------------+---------------+--------------------+
| Command (1B)   | KeyLen (4B)    | ValueLen (4B) | Key (5B)           |
注：所有多字节字段使用大端字节序（Big-Endian）
```

### 2. LRU缓存数据模型
```
LRUCache
├── capacity (int): 最大缓存条目数
├── list (*list.List): container.list双向链表（标准库实现）
├── cache (map[string]*list.Element): 哈希表
│   └── Key → list.Element指针
└── mu (sync.Mutex): 互斥锁
```

### 3. 一致性哈希环数据模型
```
HashRing
├── virtualNodes (int): 每个物理节点的虚拟节点数
├── nodeCount (int): 当前物理节点数量
├── sortedNodes ([]uint64): 已排序的哈希值数组
└── ring ([]VirtualNode): 虚拟节点数组

VirtualNode
├── NodeID (string): 物理节点ID
├── VirtualNode (string): 虚拟节点名称
├── Hash (uint64): 哈希值
└── Weight (int): 节点权重（可选）
```

### 4. 缓存节点数据模型
```
CacheNode
├── ID (string): 节点ID
├── capacity (int): 缓存容量
├── lru (*LRUCache): LRU缓存实例
├── ring (*HashRing): 所属的哈希环
├── status (string): 节点状态
├── masterID (string): 主节点ID
└── mu (sync.RWMutex): 读写锁
```

### 5. TCP连接数据模型
```
TCPServer
├── address (string): 监听地址
├── listener (net.Listener): TCP监听器
├── nodes ([]*CacheNode): 缓存节点列表
├── stopChan (chan struct{}): 停止信号通道

ClientConn
├── conn (net.Conn): TCP连接
├── nodeID (string): 关联的缓存节点ID
└── mu (sync.Mutex): 互斥锁
```

### 6. 主从复制数据模型
```
ReplicationController
├── nodes ([]*CacheNode): 所有缓存节点
└── state (*ReplicationState): 复制状态

ReplicationState
├── MasterID (string): 当前主节点ID
├── SlaveID (string): 从节点ID
├── IsMaster (bool): 是否为主节点
├── SyncedCount (int): 已同步数据条数
├── LastSyncTime (time.Time): 最后同步时间
└── mu (sync.Mutex): 互斥锁

ReplicationCommand
├── Command (uint8): 复制命令类型
├── Key (string): Key
├── Value ([]byte): Value
└── Count (uint32): 数据条数
```

---

## 技术选型说明

### 1. 编程语言：Go 1.26.4

**软件环境选择**：
- **推荐版本**：Go 1.26.4（当前环境最新稳定版）
- **最低要求**：Go 1.21+（向后兼容）

**最终使用版本**：
- 在CI/CD和生产环境中明确指定Go版本：1.26.4
- 通过`go.mod`的`go`指令锁定版本，确保构建一致性

**选择理由**：
- **高性能**：Go编译后的二进制文件运行效率高，适合网络服务
- **并发模型**：goroutine和channel提供简洁的并发编程模型
- **标准库丰富**：无需引入第三方依赖即可实现网络通信、数据结构
- **跨平台支持**：Windows/Linux/macOS均支持
- **语法简洁**：开发效率高，学习曲线平缓
- **静态类型**：编译时检查错误，减少运行时问题

**不选择方案**：
- **Java**：虚拟机启动慢，配置复杂，不适合轻量级项目
- **Python**：性能较低，网络服务延迟大，不适合高并发场景
- **C++**：开发效率低，错误多，标准库相对匮乏

---

### 2. 网络通信：Go标准库 `net`

**选择理由**：
- **标准库支持**：无需引入第三方TCP库
- **简单直接**：提供基本的TCP监听和连接管理功能
- **足够使用**：支持多客户端并发连接（通过goroutine）
- **错误处理完善**：提供详细的错误信息

**不选择方案**：
- **gRPC**：过度设计，引入Protobuf依赖，协议复杂
- **WebSocket**：HTTP升级协议，不适合TCP连接场景
- **Netty（Java）**：技术栈不一致，跨语言通信复杂

---

### 3. 数据结构：container/list + `hash/fnv`

**LRU缓存实现选择**：
- **双向链表（container/list标准库）**：
  - 选择理由：
    - 项目以学习分布式缓存原理为核心目标，container/list标准库提供了成熟的LRU算法实现基础
    - 代码简洁，减少了手动管理链表节点的复杂性
    - Go标准库维护，API稳定，便于调试和扩展
  - 不选择方案：自定义双向链表虽然能更好展示算法原理，但增加代码复杂度和维护成本
  - 复杂度：O(1)时间复杂度完成所有LRU操作

- **哈希表（Go map）**：
  - 选择理由：标准库支持，O(1)查找性能
  - 不选择方案：红黑树等其他结构过度设计，哈希表已足够

**一致性哈希实现选择**：
- **FNV-1a哈希算法（hash/fnv）**：
  - 选择理由：快速、均匀的哈希算法，标准库支持
  - 不选择方案：自定义哈希函数实现复杂且效果差

- **环形数组+指针**：
  - 选择理由：性能最优，查找O(log N)或O(1)（取决于实现方式）

---

### 4. 并发控制：`sync` 包

**选择理由**：
- **互斥锁（sync.Mutex）**：保护共享数据，简单直接
- **读写锁（sync.RWMutex）**：读多写少场景优化，提高并发性能
- **无锁数据结构**：初步版本不引入，保持代码简单

---

### 5. 序列化：`encoding/binary`

**选择理由**：
- **标准库支持**：无需引入第三方序列化库
- **高效**：二进制序列化体积小，速度快
- **简单**：API简洁，易于实现自定义协议

**不选择方案**：
- **JSON**：文本格式体积大，解析慢，不适合二进制协议
- **Protobuf**：引入依赖，配置复杂，过度设计

---

### 6. 仿真/模拟方案说明

**开发环境模拟**：
- 使用**本地多进程模拟**：
  - 多个TCP服务器进程模拟多个缓存节点
  - 使用`netcat`或自定义客户端进行集成测试
  - 使用`go test`进行单元测试

- **网络延迟模拟**：
  - 使用`tc`（Traffic Control）工具模拟网络延迟
  - 测试主从复制同步延迟

- **负载测试**：
  - 使用自定义测试脚本生成大量请求
  - 验证一致性哈希的负载分布和LRU淘汰

**性能基准测试**：
- 单节点缓存吞吐量测试（GET/SET操作）
- 一致性哈希路由性能测试
- 主从复制同步延迟测试
- 多客户端并发测试

---

## 约束要求

### 1. 功能约束

**核心功能约束**：
- [ ] 必须实现LRU缓存淘汰算法（自定义双向链表+哈希表）
- [ ] 必须实现GET/SET/DELETE/INFO命令
- [ ] 必须实现自定义二进制协议（Command+KeyLen+ValueLen+Data，大端字节序）
- [ ] 必须实现一致性哈希分片（虚拟节点机制）
- [ ] 必须实现简化版主从复制（写同步+故障切换）

**协议约束**：
- [ ] 命令码必须符合定义（GET=0x01, SET=0x02, DELETE=0x03, INFO=0x04）
- [ ] 错误码必须符合定义（SUCCESS=0x00, ERROR_UNKNOWN_COMMAND=0x01, ERROR_INVALID_KEY=0x02, ERROR_INVALID_VALUE=0x03, ERROR_CACHE_FULL=0x04）
- [ ] 对非法命令必须返回ERROR_UNKNOWN_COMMAND
- [ ] 对缺少参数的请求必须返回ERROR_INVALID_KEY
- [ ] 必须处理协议帧长度不足的情况

**缓存约束**：
- [ ] 缓存容量限制必须生效（达到上限时自动淘汰）
- [ ] LRU淘汰必须准确（最少使用者优先淘汰）
- [ ] DELETE操作必须正确更新LRU链表
- [ ] 查询不存在的Key必须返回null
- [ ] 空键必须拒绝（返回ERROR_INVALID_KEY）

**网络约束**：
- [ ] 服务器必须监听端口7000（默认）
- [ ] 必须支持多客户端并发连接（至少10个）
- [ ] 客户端异常断开必须正确处理
- [ ] 必须捕获和处理网络异常

**一致性哈希约束**：
- [ ] 必须实现虚拟节点机制（默认每个物理节点100个虚拟节点）
- [ ] 1000次SET操作后，3个分片的数据分布差异<30%
- [ ] 同一个Key必须总是路由到同一个节点
- [ ] 添加新节点后，约10-20%的数据迁移
- [ ] 移除节点后，数据必须重分配到其他节点

**主从复制约束**：
- [ ] Master的SET操作必须在10ms内同步到Slave
- [ ] Slave断开重连后必须请求全量同步
- [ ] Master故障后Slave必须提升为新的Master
- [ ] 原Master重启后必须连接到新Master并同步
- [ ] 全量同步数据必须与Master完全一致

### 2. 性能约束

- [ ] 单节点支持最多10万条缓存数据
- [ ] 支持1000+并发连接
- [ ] 缓存命中响应延迟< 1ms
- [ ] 主从复制同步延迟< 10ms
- [ ] 单进程内存使用不超过2GB

### 3. 测试约束

- [ ] 必须编写单元测试，覆盖率> 60%
- [ ] 必须编写集成测试，覆盖所有核心场景（至少20个）
- [ ] 必须测试正常场景（GET/SET/DELETE/INFO）
- [ ] 必须测试异常场景（非法命令、参数缺失、连接断开等）
- [ ] 必须测试边界条件（缓存满、空键、超大值等）

### 4. 代码质量约束

- [ ] 代码必须遵循Go规范（命名、格式、注释）
- [ ] 必须使用接口抽象模块依赖（便于测试和扩展）
- [ ] 必须使用错误处理机制，不忽略错误
- [ ] 必须注释关键代码（算法、复杂逻辑、数据结构）
- [ ] 必须提供README.md使用说明

### 5. 非功能性设计（略过）

本次设计仅关注功能设计，不涉及：
- 性能优化策略
- 安全性设计
- 可观测性（监控、日志）
- 可维护性（部署、运维）

---

## 附录：场景覆盖映射

### 1. LRU缓存场景覆盖
| 场景 | 对应功能模块 | 验收条件 |
|------|-------------|---------|
| LRU缓存基本读写操作 | Module 2 (LRUCache) | GET返回Value1, SET添加Key4, GET返回Key4 |
| 缓存达到容量上限时自动淘汰 | Module 2 (LRUCache) | 添加第101条时Key1被淘汰 |
| 重复访问热点数据保持命中 | Module 2 (LRUCache) | Key1重复访问3次，仍能命中 |
| 删除操作更新LRU链表 | Module 2 (LRUCache) | DELETE Key50后，Key50不视为最近使用 |
| 查询不存在的键值 | Module 2 (LRUCache) | GET Key999返回null |
| 空值或空键的SET操作 | Module 2 (LRUCache) | KeyEmpty=""允许，空键"="拒绝 |

### 2. TCP服务器场景覆盖
| 场景 | 对应功能模块 | 验收条件 |
|------|-------------|---------|
| 服务器正常启动和监听 | Module 5 (TCPServer) | 服务器监听端口7000 |
| 多客户端并发连接 | Module 5 (TCPServer) | 支持5个客户端并发连接 |
| 客户端异常断开连接 | Module 5 (TCPServer) | 服务器捕获异常并清理资源 |
| 协议帧长度不足 | Module 5 (TCPServer) | 服务器不崩溃，等待完整帧 |
| 非法命令处理 | Module 5 (TCPServer) | 返回ERROR_UNKNOWN_COMMAND |

### 3. 自定义协议场景覆盖
| 场景 | 对应功能模块 | 验收条件 |
|------|-------------|---------|
| GET命令正常处理 | Module 1 (Protocol) + Module 5 (Server) | 返回Command=GET, Status=SUCCESS, Value="Value1" |
| SET命令正常处理 | Module 1 (Protocol) + Module 5 (Server) | 返回Status=SUCCESS，缓存大小增加1 |
| DELETE命令正常处理 | Module 1 (Protocol) + Module 5 (Server) | 返回Status=SUCCESS，缓存大小减少1 |
| INFO命令返回服务器信息 | Module 1 (Protocol) + Module 5 (Server) | 响应包含服务器ID和版本号 |
| 无效命令返回错误码 | Module 1 (Protocol) + Module 5 (Server) | 返回ERROR_UNKNOWN_COMMAND (0x01) |
| 参数缺失或格式错误 | Module 1 (Protocol) + Module 5 (Server) | 返回ERROR_INVALID_KEY (0x02) |
| 校验码错误 | Module 1 (Protocol) | 可选处理，不崩溃 |

### 4. 一致性哈希场景覆盖
| 场景 | 对应功能模块 | 验收条件 |
|------|-------------|---------|
| 单分片基础功能 | Module 3 (HashRing) | 数据存储在Node1 |
| 虚拟节点数据均匀分布 | Module 3 (HashRing) | 1000次SET后，3个分片分布差异<30% |
| 一致性哈希环形成 | Module 3 (HashRing) | 所有Key映射到哈希环上的节点 |
| 添加新节点后的数据迁移 | Module 3 (HashRing) | 约10-20%的NodeA数据迁移到NodeD |
| 移除节点后的数据重分配 | Module 3 (HashRing) | 数据重新分配到其他节点 |
| Key的哈希冲突处理 | Module 3 (HashRing) | Key1和Key2能正确存储和读取 |

### 5. 主从复制场景覆盖
| 场景 | 对应功能模块 | 验收条件 |
|------|-------------|---------|
| 主从同步正常工作 | Module 6 (Replication) | Slave在10ms内接收到Master的数据 |
| 从节点断开重连后恢复同步 | Module 6 (Replication) | Slave请求全量同步并完成 |
| 主节点故障后从节点提升 | Module 6 (Replication) | Slave状态变为"Master"，接受写操作 |
| 主节点恢复后成为从节点 | Module 6 (Replication) | 原Master状态变为"Slave"，同步新数据 |
| 协议帧超大导致缓冲区溢出 | Module 6 (Replication) | 返回ERROR_INVALID_VALUE，不分配1GB内存 |

---

**文档版本**: v2.0
**创建日期**: 2026-06-06
**更新日期**: 2026-06-06
**作者**: [Your Name]
**状态**: 待评审
**适用范围**: SD-03分布式缓存系统功能设计

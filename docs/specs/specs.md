# 分布式缓存系统 - 功能场景规格说明

本文档定义了分布式缓存系统的核心功能场景，遵循场景驱动开发（Scenario-Driven Development）规范。

**适用范围**：仅包含核心功能模块的基础实现和验证，聚焦LRU淘汰算法、TCP服务器、自定义协议、一致性哈希分片、简化版主从复制。

---

## 1. LRU缓存淘汰策略

### Requirement: LRU缓存淘汰算法
The system SHALL implement a Least Recently Used (LRU) cache eviction policy using a combination of doubly-linked list and hash table, maintaining the most recently used items at the front and evicting the least recently used items when the cache reaches its capacity.

#### Scenario: LRU缓存基本读写操作
- GIVEN 缓存系统已初始化，初始容量为100条数据
- AND 缓存中已存在数据：Key1=Value1, Key2=Value2, Key3=Value3
- WHEN 执行GET请求查询Key1
- AND 执行SET操作添加Key4=Value4
- AND 执行GET请求查询Key4
- AND 执行GET请求查询Key2
- THEN 缓存系统 MUST 返回 Value1, Value4, Value2
- AND 缓存系统 MUST 保持数据一致性

#### Scenario: 缓存达到容量上限时自动淘汰
- GIVEN 缓存系统已初始化，初始容量为100条数据
- AND 缓存中已存在100条数据（Key1~Key100）
- WHEN 执行SET操作添加Key101=Value101
- AND 执行GET请求查询Key101
- AND 执行GET请求查询Key1
- THEN 缓存系统 MUST 返回 Value101
- AND 缓存系统 MUST NOT 返回 Key1（被LRU淘汰）
- AND 缓存系统 MUST 确保缓存大小保持为100条

#### Scenario: 重复访问热点数据保持命中
- GIVEN 缓存系统已初始化，容量为100条数据
- AND 缓存中已存在Key1=Value1, Key2=Value2, Key3=Value3
- WHEN 执行GET请求查询Key1，重复3次
- AND 执行SET操作添加Key4=Value4
- AND 再次执行GET请求查询Key1
- THEN 缓存系统 MUST 返回 Value1 每次查询
- AND 缓存系统 MUST 成功添加Key4

#### Scenario: 删除操作更新LRU链表
- GIVEN 缓存系统已初始化，容量为100条数据
- AND 缓存中已存在Key1~Key100
- WHEN 执行DELETE操作删除Key50
- AND 执行GET请求查询Key50
- AND 执行GET请求查询Key51
- AND 执行SET操作添加Key101=Value101
- THEN 缓存系统 MUST NOT 返回 Key50
- AND 缓存系统 MUST 返回 Key51

#### Scenario: 查询不存在的键值
- GIVEN 缓存系统已初始化，容量为100条数据
- AND 缓存中已存在Key1~Key50
- WHEN 执行GET请求查询Key999
- THEN 缓存系统 MUST 返回 null
- AND 缓存系统 MUST NOT 改变缓存大小

#### Scenario: 空值或空键的SET操作
- GIVEN 缓存系统已初始化，容量为100条数据
- WHEN 执行SET操作添加KeyEmpty=""
- AND 执行SET操作添加""=ValueEmpty
- THEN 缓存系统 MAY 返回成功（空值允许）
- AND 缓存系统 MUST NOT 允许空键（返回错误）

#### Scenario: 超大值的SET操作
- GIVEN 缓存系统已初始化，容量为100条数据
- WHEN 执行SET操作添加KeyLarge="data_large"（长度>1MB）
- THEN 缓存系统 MAY 返回错误（可选）
- AND 缓存系统 MUST NOT 分配超过缓冲区大小的内存

---

## 2. TCP服务器实现

### Requirement: TCP服务器基本功能
The system SHALL provide a TCP server that supports multiple concurrent client connections and handles network exceptions gracefully.

#### Scenario: 服务器正常启动和监听
- GIVEN 缓存系统启动
- WHEN 执行服务器启动命令
- THEN 服务器 MUST 成功监听指定端口（默认7000）
- AND 服务器 MUST 保持运行状态

#### Scenario: 多客户端并发连接
- GIVEN 缓存服务器已启动并监听端口7000
- WHEN 执行客户端连接请求5次
- AND 每个客户端发送独立的GET请求
- THEN 服务器 MUST 成功处理所有5个连接
- AND 每个客户端 MUST 能够正常读写数据

#### Scenario: 客户端异常断开连接
- GIVEN 缓存服务器已启动并连接了3个客户端
- WHEN 执行客户端强制关闭网络连接
- THEN 服务器 MUST 捕获连接异常
- AND 服务器 MUST 清理该客户端的资源
- AND 其他客户端连接 MUST 不受影响

#### Scenario: 协议帧长度不足
- GIVEN 缓存服务器已启动
- WHEN 发送一个不完整的帧（只发送帧头，不发送数据）
- THEN 服务器 MAY 等待直到超时或发送完整帧
- AND 服务器 MUST NOT 崩溃

#### Scenario: 非法命令处理
- GIVEN 缓存服务器已启动
- WHEN 发送二进制协议请求：Command=0x99（未知命令）
- THEN 服务器 MUST 返回错误码 ERROR_UNKNOWN_COMMAND
- AND 服务器 MUST NOT 崩溃

---

## 3. 自定义协议设计

### Requirement: 自定义二进制协议
The system SHALL define a simple binary protocol for communication between cache server and clients, supporting basic commands like GET, SET, DELETE, and INFO.

#### Scenario: GET命令正常处理
- GIVEN 缓存服务器已启动，缓存中包含Key1=Value1
- WHEN 发送二进制协议请求：Command=GET, Key="Key1"
- THEN 服务器 MUST 返回：Command=GET, Status=SUCCESS, Value="Value1"
- AND 服务器 MUST 验证协议帧长度正确

#### Scenario: SET命令正常处理
- GIVEN 缓存服务器已启动，容量为10000
- WHEN 发送二进制协议请求：Command=SET, Key="TestKey", Value="TestValue"
- THEN 服务器 MUST 返回：Command=SET, Status=SUCCESS
- AND 缓存系统 MUST 存储Key和Value
- AND 缓存大小 MUST 增加1

#### Scenario: DELETE命令正常处理
- GIVEN 缓存服务器已启动，缓存中包含Key1=Value1
- WHEN 发送二进制协议请求：Command=DELETE, Key="Key1"
- THEN 服务器 MUST 返回：Command=DELETE, Status=SUCCESS
- AND 缓存系统 MUST 不再包含Key1
- AND 缓存大小 MUST 减少1

#### Scenario: INFO命令返回服务器信息
- GIVEN 缓存服务器已启动
- WHEN 发送二进制协议请求：Command=INFO
- THEN 服务器 MUST 返回：Command=INFO, Status=SUCCESS
- AND 响应 MUST 包含服务器ID和版本号

#### Scenario: 无效命令返回错误码
- GIVEN 缓存服务器已启动
- WHEN 发送二进制协议请求：Command=0x99（未知命令）
- THEN 服务器 MUST 返回：Status=ERROR_UNKNOWN_COMMAND
- AND 错误码 MUST 为0x01

#### Scenario: 参数缺失或格式错误
- GIVEN 缓存服务器已启动
- WHEN 发送二进制协议请求：Command=GET（缺少Key参数）
- THEN 服务器 MUST 返回：Status=ERROR_INVALID_KEY
- AND 错误码 MUST 为0x02

#### Scenario: 校验码错误
- GIVEN 缓存服务器已启动
- WHEN 发送二进制协议请求，Key长度字段为100但实际Key长度为50
- THEN 服务器 MAY 返回错误（可选）
- AND 服务器 MUST NOT 崩溃

---

## 4. 一致性哈希分片

### Requirement: 一致性哈希分片
The system SHALL implement consistent hashing to distribute cache data across multiple shards, using virtual nodes to achieve basic load balancing.

#### Scenario: 单分片基础功能
- GIVEN 缓存系统初始化，仅包含一个分片节点（Node1）
- WHEN 执行GET请求Key1
- AND 执行SET操作Key1=Value1
- THEN 数据 MUST 存储在Node1的分片上
- AND 所有操作 MUST 在Node1上执行

#### Scenario: 虚拟节点数据均匀分布
- GIVEN 缓存系统初始化，包含3个物理节点（NodeA、NodeB、NodeC）
- AND 每个物理节点有100个虚拟节点
- WHEN 执行1000次SET操作，随机生成1000个不同的Key
- THEN 节点间的数据分布差异 MAY < 30%
- AND 总数据量 MUST 为1000条

#### Scenario: 一致性哈希环环形成
- GIVEN 缓存系统初始化，添加3个物理节点
- WHEN 执行查询每个Key所属的分片节点
- THEN 所有Key MUST 映射到哈希环上的某个节点
- AND 同一个Key MUST 映射到同一个节点

#### Scenario: 添加新节点后的数据迁移
- GIVEN 缓存系统已包含NodeA、NodeB、NodeC
- AND NodeA上已存储500条数据
- WHEN 添加新节点NodeD（虚拟节点数100）
- THEN 约10-20%的NodeA数据 MAY 迁移到NodeD
- AND 其余数据 MUST 保持在NodeA
- AND NodeD MUST 接收约100-200条数据

#### Scenario: 移除节点后的数据重分配
- GIVEN 缓存系统包含NodeA、NodeB、NodeC、NodeD
- AND NodeD上存储约150条数据
- WHEN 移除NodeD
- THEN 约150条数据 MAY 重新分配到其他3个节点
- AND 重分配后系统 MUST 仍然可以正常读写

#### Scenario: Key的哈希冲突处理
- GIVEN 缓存系统初始化
- WHEN 执行SET操作Key1=Value1，再执行SET操作Key2=Value2
- THEN 两个Key MUST 能够正确存储
- AND 读取Key1 MUST 返回Value1，读取Key2 MUST 返回Value2

---

## 5. 主从复制

### Requirement: 简化版主从复制
The system SHALL implement a simplified primary-secondary replication mechanism where write operations on the primary are synchronized to one or more secondary nodes.

#### Scenario: 主从同步正常工作
- GIVEN 缓存系统包含1个主节点（Master）和1个从节点（Slave）
- AND 主从节点已建立连接
- WHEN 在Master上执行SET操作Key1=Value1
- THEN Master MUST 立即返回成功
- AND Slave MUST 在10ms内接收到Key1=Value1
- AND Slave MUST 存储Key1=Value1

#### Scenario: 从节点断开重连后恢复同步
- GIVEN 缓存系统包含1个Master和1个Slave
- AND Master上有1000条数据
- WHEN 在Slave断开连接10秒后重新连接
- THEN Slave MUST 与Master建立连接
- AND Slave MUST 请求同步Master的所有1000条数据
- AND Slave MUST 完成同步

#### Scenario: 主节点故障后从节点提升
- GIVEN 缓存系统包含1个主节点（Master）和1个从节点（Slave）
- AND 主从节点已同步所有数据
- WHEN Master进程崩溃
- AND Slave检测到Master不可达
- THEN Slave状态 MUST 变为"Master"
- AND Slave MUST 开始接受写操作

#### Scenario: 主节点恢复后成为从节点
- GIVEN 缓存系统包含1个主节点（Master）和1个从节点（Slave）
- AND Master故障后Slave提升为新Master
- AND Slave上有500条新数据
- WHEN 原Master重启并连接到Slave
- THEN 原Master状态 MUST 变为"Slave"
- AND 原Master MUST 请求同步新Master的数据

#### Scenario: 协议帧超大导致缓冲区溢出
- GIVEN 缓存服务器已启动
- WHEN 发送一个超过缓冲区大小的帧（例如Key长度为1GB）
- THEN 服务器 MUST 返回错误：ERROR_INVALID_VALUE
- AND 服务器 MUST NOT 分配1GB内存

---

## 6. 集成测试场景

### Requirement: 端到端集成验证
The system SHALL support end-to-end integration testing to verify the complete workflow from client request to server response.

#### Scenario: 完整的缓存读写流程
- GIVEN 缓存系统已启动，包含3个分片节点
- AND 缓存容量为100条数据
- WHEN 客户端执行SET操作Key1=Value1
- AND 执行GET操作Key1，期望返回Value1
- AND 执行SET操作Key2=Value2
- AND 执行GET操作Key2，期望返回Value2
- AND 执行DELETE操作Key1
- AND 执行GET操作Key1，期望返回null
- THEN 所有操作 MUST 成功执行
- AND 缓存数据 MUST 一致
- AND LRU链表 MUST 正确更新

---

**文档版本**: v2.1
**创建日期**: 2026-06-06
**更新日期**: 2026-06-09
**作者**: SD-03项目组
**状态**: 已验证
**适用范围**: 核心功能模块基础实现和验证，不包含性能测试、错误恢复、安全性等进阶功能

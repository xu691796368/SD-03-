// Package node_test 实现缓存节点模块的单元测试（Task 3.4）
// 覆盖：缓存节点增删改查、状态管理、模块集成（LRU + 一致性哈希）
package node_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/yourusername/sd-03-cache/pkg/node"
	"github.com/yourusername/sd-03-cache/pkg/shard"
)

// ============ 辅助函数 ============

// newTestNode 创建测试用缓存节点（容量100，已启动）
func newTestNode(t *testing.T, id string) *node.CacheNode {
	t.Helper()
	n, err := node.NewCacheNode(id, 100)
	if err != nil {
		t.Fatalf("NewCacheNode(%q, 100) 失败: %v", id, err)
	}
	if err := n.Start(); err != nil {
		t.Fatalf("Start() 失败: %v", err)
	}
	return n
}

// newTestNodeWithRing 创建带哈希环的测试用缓存节点
func newTestNodeWithRing(t *testing.T, id string) (*node.CacheNode, *shard.HashRing) {
	t.Helper()
	n := newTestNode(t, id)
	ring, err := shard.NewHashRing(100)
	if err != nil {
		t.Fatalf("NewHashRing(100) 失败: %v", err)
	}
	if err := n.Init(ring); err != nil {
		t.Fatalf("Init(ring) 失败: %v", err)
	}
	return n, ring
}

// ============================================================
// 1. 构造函数测试
// ============================================================

// TestNewCacheNode_Success 测试正常创建缓存节点
func TestNewCacheNode_Success(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		capacity int
	}{
		{"节点A-容量100", "NodeA", 100},
		{"节点B-容量1", "NodeB", 1},
		{"节点C-容量10000", "NodeC", 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := node.NewCacheNode(tt.id, tt.capacity)
			if err != nil {
				t.Fatalf("NewCacheNode(%q, %d) 不应返回错误，得到: %v", tt.id, tt.capacity, err)
			}
			if n == nil {
				t.Fatal("NewCacheNode 不应返回 nil")
			}
			if n.GetNodeID() != tt.id {
				t.Errorf("GetNodeID() = %q, 期望 %q", n.GetNodeID(), tt.id)
			}
			if n.GetCapacity() != tt.capacity {
				t.Errorf("GetCapacity() = %d, 期望 %d", n.GetCapacity(), tt.capacity)
			}
			if n.GetStatus() != node.StatusStopped {
				t.Errorf("新节点状态 = %q, 期望 %q", n.GetStatus(), node.StatusStopped)
			}
			if n.Size() != 0 {
				t.Errorf("新节点 Size() = %d, 期望 0", n.Size())
			}
		})
	}
}

// TestNewCacheNode_EmptyID 测试空节点ID
func TestNewCacheNode_EmptyID(t *testing.T) {
	n, err := node.NewCacheNode("", 100)
	if n != nil {
		t.Error("空ID应返回 nil 节点")
	}
	if err != node.ErrEmptyID {
		t.Errorf("空ID错误 = %v, 期望 ErrEmptyID", err)
	}
}

// TestNewCacheNode_InvalidCapacity 测试无效容量
func TestNewCacheNode_InvalidCapacity(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
	}{
		{"容量为0", 0},
		{"容量为负数", -1},
		{"容量为-100", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := node.NewCacheNode("TestNode", tt.capacity)
			if n != nil {
				t.Errorf("容量 %d 应返回 nil 节点", tt.capacity)
			}
			if err != node.ErrInvalidCapacity {
				t.Errorf("容量 %d 错误 = %v, 期望 ErrInvalidCapacity", tt.capacity, err)
			}
		})
	}
}

// ============================================================
// 2. 缓存增删改查测试
// ============================================================

// TestGetSet_BasicOperation 测试基本Get/Set操作
func TestGetSet_BasicOperation(t *testing.T) {
	n := newTestNode(t, "Node1")

	// Set 操作
	if err := n.Set("key1", []byte("value1")); err != nil {
		t.Fatalf("Set(key1, value1) 失败: %v", err)
	}
	if err := n.Set("key2", []byte("value2")); err != nil {
		t.Fatalf("Set(key2, value2) 失败: %v", err)
	}

	// Get 操作
	val, err := n.Get("key1")
	if err != nil {
		t.Fatalf("Get(key1) 失败: %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("Get(key1) = %q, 期望 %q", val, "value1")
	}

	val, err = n.Get("key2")
	if err != nil {
		t.Fatalf("Get(key2) 失败: %v", err)
	}
	if string(val) != "value2" {
		t.Errorf("Get(key2) = %q, 期望 %q", val, "value2")
	}

	if n.Size() != 2 {
		t.Errorf("Size() = %d, 期望 2", n.Size())
	}
}

// TestGet_NonExistentKey 测试获取不存在的Key
func TestGet_NonExistentKey(t *testing.T) {
	n := newTestNode(t, "Node1")

	n.Set("key1", []byte("value1"))

	val, err := n.Get("key_not_exist")
	if err != nil {
		t.Fatalf("Get(不存在的key) 不应返回错误: %v", err)
	}
	if val != nil {
		t.Errorf("Get(不存在的key) = %v, 期望 nil", val)
	}
}

// TestSet_UpdateExisting 测试更新已存在的Key
func TestSet_UpdateExisting(t *testing.T) {
	n := newTestNode(t, "Node1")

	n.Set("key1", []byte("value1"))
	n.Set("key1", []byte("updated_value"))

	val, err := n.Get("key1")
	if err != nil {
		t.Fatalf("Get(key1) 失败: %v", err)
	}
	if string(val) != "updated_value" {
		t.Errorf("更新后 Get(key1) = %q, 期望 %q", val, "updated_value")
	}

	// 更新不应增加大小
	if n.Size() != 1 {
		t.Errorf("更新后 Size() = %d, 期望 1", n.Size())
	}
}

// TestDelete_Success 测试删除存在的Key
func TestDelete_Success(t *testing.T) {
	n := newTestNode(t, "Node1")

	n.Set("key1", []byte("value1"))
	n.Set("key2", []byte("value2"))

	if err := n.Delete("key1"); err != nil {
		t.Fatalf("Delete(key1) 失败: %v", err)
	}

	val, err := n.Get("key1")
	if err != nil {
		t.Fatalf("Delete后 Get(key1) 失败: %v", err)
	}
	if val != nil {
		t.Errorf("Delete后 Get(key1) = %v, 期望 nil", val)
	}

	if n.Size() != 1 {
		t.Errorf("Delete后 Size() = %d, 期望 1", n.Size())
	}
}

// TestDelete_NonExistentKey 测试删除不存在的Key
func TestDelete_NonExistentKey(t *testing.T) {
	n := newTestNode(t, "Node1")

	// 删除不存在的Key不应报错
	if err := n.Delete("not_exist"); err != nil {
		t.Errorf("Delete(不存在的key) 不应返回错误: %v", err)
	}
}

// TestSet_EmptyKey 测试空键Set操作
func TestSet_EmptyKey(t *testing.T) {
	n := newTestNode(t, "Node1")

	err := n.Set("", []byte("value"))
	if err == nil {
		t.Error("Set(empty_key) 应返回错误")
	}
}

// TestSet_EmptyValue 测试空值Set操作
func TestSet_EmptyValue(t *testing.T) {
	n := newTestNode(t, "Node1")

	err := n.Set("key_empty", []byte{})
	if err != nil {
		t.Errorf("Set(key, empty_value) 不应返回错误: %v", err)
	}

	val, err := n.Get("key_empty")
	if err != nil {
		t.Fatalf("Get(key_empty) 失败: %v", err)
	}
	if len(val) != 0 {
		t.Errorf("Get(key_empty) 长度 = %d, 期望 0", len(val))
	}
}

// TestSet_LargeValue 测试大值Set操作
func TestSet_LargeValue(t *testing.T) {
	n := newTestNode(t, "Node1")

	largeValue := make([]byte, 1024*100) // 100KB
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	err := n.Set("large_key", largeValue)
	if err != nil {
		t.Errorf("Set(large_key, 100KB) 不应返回错误: %v", err)
	}

	val, err := n.Get("large_key")
	if err != nil {
		t.Fatalf("Get(large_key) 失败: %v", err)
	}
	if len(val) != len(largeValue) {
		t.Errorf("Get(large_key) 长度 = %d, 期望 %d", len(val), len(largeValue))
	}
}

// TestCRUD_MultipleOperations 测试多次增删改查
func TestCRUD_MultipleOperations(t *testing.T) {
	n := newTestNode(t, "Node1")

	// 批量写入
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		if err := n.Set(key, []byte(value)); err != nil {
			t.Fatalf("Set(%s) 失败: %v", key, err)
		}
	}

	if n.Size() != 50 {
		t.Errorf("批量Set后 Size() = %d, 期望 50", n.Size())
	}

	// 批量读取验证
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key_%d", i)
		expected := fmt.Sprintf("value_%d", i)
		val, err := n.Get(key)
		if err != nil {
			t.Fatalf("Get(%s) 失败: %v", key, err)
		}
		if string(val) != expected {
			t.Errorf("Get(%s) = %q, 期望 %q", key, val, expected)
		}
	}

	// 批量删除
	for i := 0; i < 25; i++ {
		key := fmt.Sprintf("key_%d", i)
		if err := n.Delete(key); err != nil {
			t.Fatalf("Delete(%s) 失败: %v", key, err)
		}
	}

	if n.Size() != 25 {
		t.Errorf("批量Delete后 Size() = %d, 期望 25", n.Size())
	}

	// 验证删除后的key不存在
	for i := 0; i < 25; i++ {
		key := fmt.Sprintf("key_%d", i)
		val, _ := n.Get(key)
		if val != nil {
			t.Errorf("Delete后 Get(%s) = %v, 期望 nil", key, val)
		}
	}

	// 验证未删除的key仍存在
	for i := 25; i < 50; i++ {
		key := fmt.Sprintf("key_%d", i)
		val, err := n.Get(key)
		if err != nil {
			t.Fatalf("Get(%s) 失败: %v", key, err)
		}
		if val == nil {
			t.Errorf("Get(%s) = nil, 期望非nil", key)
		}
	}
}

// ============================================================
// 3. 状态管理测试
// ============================================================

// TestNodeStatus_Transition 测试节点状态转换
func TestNodeStatus_Transition(t *testing.T) {
	n, err := node.NewCacheNode("Node1", 100)
	if err != nil {
		t.Fatalf("NewCacheNode 失败: %v", err)
	}

	// 初始状态为 Stopped
	if n.GetStatus() != node.StatusStopped {
		t.Errorf("初始状态 = %q, 期望 %q", n.GetStatus(), node.StatusStopped)
	}

	// Start -> Running
	if err := n.Start(); err != nil {
		t.Fatalf("Start() 失败: %v", err)
	}
	if n.GetStatus() != node.StatusRunning {
		t.Errorf("Start后状态 = %q, 期望 %q", n.GetStatus(), node.StatusRunning)
	}

	// 重复 Start（幂等）
	if err := n.Start(); err != nil {
		t.Fatalf("重复 Start() 失败: %v", err)
	}

	// Stop -> Stopped
	if err := n.Stop(); err != nil {
		t.Fatalf("Stop() 失败: %v", err)
	}
	if n.GetStatus() != node.StatusStopped {
		t.Errorf("Stop后状态 = %q, 期望 %q", n.GetStatus(), node.StatusStopped)
	}

	// 重复 Stop（幂等）
	if err := n.Stop(); err != nil {
		t.Fatalf("重复 Stop() 失败: %v", err)
	}
}

// TestNodeStatus_SetStatus 测试通过SetStatus设置各种状态
func TestNodeStatus_SetStatus(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	validStatuses := []string{
		node.StatusStopped,
		node.StatusRunning,
		node.StatusMaster,
		node.StatusSlave,
	}

	for _, status := range validStatuses {
		if err := n.SetStatus(status); err != nil {
			t.Errorf("SetStatus(%q) 不应返回错误: %v", status, err)
		}
		if n.GetStatus() != status {
			t.Errorf("SetStatus(%q) 后状态 = %q, 期望 %q", status, n.GetStatus(), status)
		}
	}
}

// TestNodeStatus_InvalidStatus 测试无效状态
func TestNodeStatus_InvalidStatus(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	invalidStatuses := []string{
		"",
		"Unknown",
		"STARTED",
		"running",
		"master",
	}

	for _, status := range invalidStatuses {
		err := n.SetStatus(status)
		if err != node.ErrInvalidStatus {
			t.Errorf("SetStatus(%q) 错误 = %v, 期望 ErrInvalidStatus", status, err)
		}
	}
}

// TestNodeStatus_StoppedNodeRejectOps 测试已停止节点拒绝操作
func TestNodeStatus_StoppedNodeRejectOps(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	// 节点未启动，操作应被拒绝
	_, err := n.Get("key1")
	if err != node.ErrNodeStopped {
		t.Errorf("Stopped状态 Get 错误 = %v, 期望 ErrNodeStopped", err)
	}

	err = n.Set("key1", []byte("value1"))
	if err != node.ErrNodeStopped {
		t.Errorf("Stopped状态 Set 错误 = %v, 期望 ErrNodeStopped", err)
	}

	err = n.Delete("key1")
	if err != node.ErrNodeStopped {
		t.Errorf("Stopped状态 Delete 错误 = %v, 期望 ErrNodeStopped", err)
	}
}

// TestNodeStatus_MasterSlave 测试Master/Slave状态切换
func TestNodeStatus_MasterSlave(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	// 设置为 Master
	if err := n.SetStatus(node.StatusMaster); err != nil {
		t.Fatalf("SetStatus(Master) 失败: %v", err)
	}
	n.Start() // Master 需要启动

	n.Set("key1", []byte("value1"))
	val, err := n.Get("key1")
	if err != nil || string(val) != "value1" {
		t.Errorf("Master节点操作失败: val=%q, err=%v", val, err)
	}

	// 设置为 Slave
	if err := n.SetStatus(node.StatusSlave); err != nil {
		t.Fatalf("SetStatus(Slave) 失败: %v", err)
	}

	// Slave节点应仍可操作
	val, err = n.Get("key1")
	if err != nil || string(val) != "value1" {
		t.Errorf("Slave节点读取失败: val=%q, err=%v", val, err)
	}
}

// TestNodeStatus_SetMasterID 测试设置主节点ID
func TestNodeStatus_SetMasterID(t *testing.T) {
	n, _ := node.NewCacheNode("Slave1", 100)

	if n.GetMasterID() != "" {
		t.Errorf("初始 MasterID = %q, 期望空字符串", n.GetMasterID())
	}

	n.SetMasterID("Master1")
	if n.GetMasterID() != "Master1" {
		t.Errorf("SetMasterID后 MasterID = %q, 期望 %q", n.GetMasterID(), "Master1")
	}
}

// ============================================================
// 4. 初始化与哈希环集成测试
// ============================================================

// TestInit_Success 测试正常初始化
func TestInit_Success(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)
	ring, _ := shard.NewHashRing(100)

	if err := n.Init(ring); err != nil {
		t.Fatalf("Init(ring) 失败: %v", err)
	}

	if n.GetRing() != ring {
		t.Error("Init后 GetRing() 应返回传入的哈希环")
	}
}

// TestInit_NilRing 测试空哈希环
func TestInit_NilRing(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	err := n.Init(nil)
	if err != node.ErrNilRing {
		t.Errorf("Init(nil) 错误 = %v, 期望 ErrNilRing", err)
	}
}

// TestInit_GetInfoWithRing 测试初始化后GetInfo包含哈希环信息
func TestInit_GetInfoWithRing(t *testing.T) {
	n, ring := newTestNodeWithRing(t, "Node1")
	ring.AddNode("Node1")

	info := n.GetInfo()

	if info["id"] != "Node1" {
		t.Errorf("GetInfo().id = %v, 期望 Node1", info["id"])
	}
	if info["status"] != node.StatusRunning {
		t.Errorf("GetInfo().status = %v, 期望 Running", info["status"])
	}
	if info["capacity"] != 100 {
		t.Errorf("GetInfo().capacity = %v, 期望 100", info["capacity"])
	}
	if info["ringNodes"] != 1 {
		t.Errorf("GetInfo().ringNodes = %v, 期望 1", info["ringNodes"])
	}
	if info["size"] != 0 {
		t.Errorf("GetInfo().size = %v, 期望 0", info["size"])
	}
}

// ============================================================
// 5. LRU 淘汰机制集成测试
// ============================================================

// TestLRUEviction_ViaNode 测试通过CacheNode触发的LRU淘汰
func TestLRUEviction_ViaNode(t *testing.T) {
	// 创建容量为5的节点
	n, err := node.NewCacheNode("Node1", 5)
	if err != nil {
		t.Fatalf("NewCacheNode 失败: %v", err)
	}
	n.Start()

	// 填满缓存
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key_%d", i)
		n.Set(key, []byte(fmt.Sprintf("value_%d", i)))
	}

	if n.Size() != 5 {
		t.Errorf("填满后 Size() = %d, 期望 5", n.Size())
	}

	// 添加第6个条目，应触发淘汰
	n.Set("key_5", []byte("value_5"))

	// key_0 应被淘汰（最久未使用）
	val, _ := n.Get("key_0")
	if val != nil {
		t.Error("key_0 应被LRU淘汰")
	}

	// key_5 应存在
	val, _ = n.Get("key_5")
	if string(val) != "value_5" {
		t.Errorf("Get(key_5) = %q, 期望 value_5", val)
	}

	if n.Size() != 5 {
		t.Errorf("淘汰后 Size() = %d, 期望 5", n.Size())
	}
}

// TestLRUEviction_HotDataKept 测试热点数据在淘汰时保留
func TestLRUEviction_HotDataKept(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 3)
	n.Start()

	n.Set("A", []byte("1"))
	n.Set("B", []byte("2"))
	n.Set("C", []byte("3"))

	// 访问A使其成为热点
	n.Get("A")
	n.Get("A")

	// 添加D，应淘汰B（最久未使用）
	n.Set("D", []byte("4"))

	val, _ := n.Get("A")
	if string(val) != "1" {
		t.Error("热点数据A应保留")
	}

	val, _ = n.Get("B")
	if val != nil {
		t.Error("B应被淘汰")
	}

	val, _ = n.Get("D")
	if string(val) != "4" {
		t.Error("D应存在")
	}
}

// ============================================================
// 6. 一致性哈希集成测试
// ============================================================

// TestHashRingIntegration_Routing 测试节点与哈希环集成路由
func TestHashRingIntegration_Routing(t *testing.T) {
	// 创建3个缓存节点
	nodes := make([]*node.CacheNode, 3)
	nodeIDs := []string{"NodeA", "NodeB", "NodeC"}

	for i, id := range nodeIDs {
		n, err := node.NewCacheNode(id, 100)
		if err != nil {
			t.Fatalf("NewCacheNode(%s) 失败: %v", id, err)
		}
		n.Start()
		nodes[i] = n
	}

	// 创建共享哈希环
	ring, _ := shard.NewHashRing(100)
	for _, id := range nodeIDs {
		ring.AddNode(id)
	}

	// 初始化每个节点使用相同的哈希环
	for _, n := range nodes {
		n.Init(ring)
	}

	// 使用哈希环路由写入数据到对应节点
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("user_%d", i)
		value := fmt.Sprintf("data_%d", i)
		targetNodeID := ring.GetNode(key)

		// 找到对应的缓存节点
		for _, n := range nodes {
			if n.GetNodeID() == targetNodeID {
				n.Set(key, []byte(value))
				break
			}
		}
	}

	// 验证数据总量
	totalSize := 0
	for _, n := range nodes {
		totalSize += n.Size()
	}
	if totalSize != 100 {
		t.Errorf("总数据量 = %d, 期望 100", totalSize)
	}

	// 验证通过哈希环路由可以正确读取
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("user_%d", i)
		expected := fmt.Sprintf("data_%d", i)
		targetNodeID := ring.GetNode(key)

		for _, n := range nodes {
			if n.GetNodeID() == targetNodeID {
				val, err := n.Get(key)
				if err != nil {
					t.Fatalf("Get(%s) 失败: %v", key, err)
				}
				if string(val) != expected {
					t.Errorf("Get(%s) = %q, 期望 %q", key, val, expected)
				}
				break
			}
		}
	}

	// 输出分布信息
	t.Logf("数据分布:")
	for _, n := range nodes {
		t.Logf("  %s: %d 条数据", n.GetNodeID(), n.Size())
	}
}

// TestHashRingIntegration_AddNode 测试动态添加节点
func TestHashRingIntegration_AddNode(t *testing.T) {
	// 初始2个节点
	nodes := make(map[string]*node.CacheNode)
	ring, _ := shard.NewHashRing(100)

	for _, id := range []string{"NodeA", "NodeB"} {
		n, _ := node.NewCacheNode(id, 200)
		n.Start()
		n.Init(ring)
		nodes[id] = n
		ring.AddNode(id)
	}

	// 写入100条数据
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		targetID := ring.GetNode(key)
		nodes[targetID].Set(key, []byte(fmt.Sprintf("val_%d", i)))
	}

	// 记录添加前的分布
	beforeA := nodes["NodeA"].Size()
	beforeB := nodes["NodeB"].Size()
	t.Logf("添加前: NodeA=%d, NodeB=%d", beforeA, beforeB)

	// 添加第三个节点
	nC, _ := node.NewCacheNode("NodeC", 200)
	nC.Start()
	nC.Init(ring)
	nodes["NodeC"] = nC
	ring.AddNode("NodeC")

	// 将数据迁移到新节点
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		targetID := ring.GetNode(key)
		// 如果数据应路由到 NodeC，从原节点读取并写入 NodeC
		if targetID == "NodeC" {
			// 从原节点读取
			for _, id := range []string{"NodeA", "NodeB"} {
				val, _ := nodes[id].Get(key)
				if val != nil {
					nC.Set(key, val)
					nodes[id].Delete(key)
					break
				}
			}
		}
	}

	t.Logf("迁移后: NodeA=%d, NodeB=%d, NodeC=%d",
		nodes["NodeA"].Size(), nodes["NodeB"].Size(), nodes["NodeC"].Size())

	// NodeC应有数据
	if nC.Size() == 0 {
		t.Error("添加NodeC后，应有数据迁移到NodeC")
	}
}

// TestHashRingIntegration_RemoveNode 测试移除节点后的路由
func TestHashRingIntegration_RemoveNode(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	// 确保移除前所有Key可路由
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		nodeID := ring.GetNode(key)
		if nodeID == "" {
			t.Errorf("Key %s 应路由到某个节点", key)
		}
	}

	// 移除NodeB
	ring.RemoveNode("NodeB")

	// 确保移除后所有Key路由到A或C
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		nodeID := ring.GetNode(key)
		if nodeID == "NodeB" {
			t.Errorf("移除NodeB后，Key %s 不应路由到NodeB", key)
		}
		if nodeID != "NodeA" && nodeID != "NodeC" {
			t.Errorf("Key %s 路由到未知节点 %s", key, nodeID)
		}
	}
}

// ============================================================
// 7. GetInfo 测试
// ============================================================

// TestGetInfo_Complete 测试GetInfo返回完整信息
func TestGetInfo_Complete(t *testing.T) {
	n, _ := node.NewCacheNode("TestNode", 50)
	n.Start()
	n.Set("k1", []byte("v1"))
	n.Set("k2", []byte("v2"))

	info := n.GetInfo()

	if info["id"] != "TestNode" {
		t.Errorf("id = %v, 期望 TestNode", info["id"])
	}
	if info["status"] != node.StatusRunning {
		t.Errorf("status = %v, 期望 Running", info["status"])
	}
	if info["capacity"] != 50 {
		t.Errorf("capacity = %v, 期望 50", info["capacity"])
	}
	if info["size"] != 2 {
		t.Errorf("size = %v, 期望 2", info["size"])
	}
	if info["isFull"] != false {
		t.Errorf("isFull = %v, 期望 false", info["isFull"])
	}
	if info["ringNodes"] != 0 {
		t.Errorf("ringNodes = %v, 期望 0（未初始化哈希环）", info["ringNodes"])
	}
}

// TestGetInfo_WithRingAndMaster 测试GetInfo包含哈希环和主节点信息
func TestGetInfo_WithRingAndMaster(t *testing.T) {
	n, ring := newTestNodeWithRing(t, "SlaveNode")
	ring.AddNode("MasterNode")
	ring.AddNode("SlaveNode")
	n.SetStatus(node.StatusSlave)
	n.SetMasterID("MasterNode")

	info := n.GetInfo()

	if info["ringNodes"] != 2 {
		t.Errorf("ringNodes = %v, 期望 2", info["ringNodes"])
	}
	if info["masterID"] != "MasterNode" {
		t.Errorf("masterID = %v, 期望 MasterNode", info["masterID"])
	}
	if info["status"] != node.StatusSlave {
		t.Errorf("status = %v, 期望 Slave", info["status"])
	}
}

// ============================================================
// 8. 并发安全测试
// ============================================================

// TestConcurrentReadWrite 测试并发读写安全
func TestConcurrentReadWrite(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 1000)
	n.Start()

	var wg sync.WaitGroup
	errChan := make(chan error, 200)

	// 并发写
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				if err := n.Set(key, []byte(key)); err != nil {
					errChan <- fmt.Errorf("Set(%s) 错误: %v", key, err)
					return
				}
			}
		}(g)
	}

	// 并发读
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				n.Get(key)
			}
		}(g)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("并发错误: %v", err)
	}
}

// TestConcurrentReadWriteDelete 测试并发读写删
func TestConcurrentReadWriteDelete(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 200)
	n.Start()

	// 预填充
	for i := 0; i < 100; i++ {
		n.Set(fmt.Sprintf("K%d", i), []byte(fmt.Sprintf("V%d", i)))
	}

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("K%d", id%100)
			switch id % 3 {
			case 0:
				n.Get(key)
			case 1:
				n.Set(key, []byte(fmt.Sprintf("V%d_new", id)))
			case 2:
				n.Delete(key)
			}
		}(i)
	}

	wg.Wait()
	// 不崩溃即通过
}

// TestConcurrentStatusChange 测试并发状态切换
func TestConcurrentStatusChange(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				n.Start()
			} else {
				n.Stop()
			}
		}(i)
	}
	wg.Wait()
	// 不崩溃即通过
}

// ============================================================
// 9. 边界条件测试
// ============================================================

// TestNode_CapacityOne 测试容量为1的节点
func TestNode_CapacityOne(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 1)
	n.Start()

	n.Set("A", []byte("ValA"))
	val, _ := n.Get("A")
	if string(val) != "ValA" {
		t.Errorf("Get(A) = %q, 期望 ValA", val)
	}

	// 添加B应淘汰A
	n.Set("B", []byte("ValB"))
	val, _ = n.Get("A")
	if val != nil {
		t.Error("容量1时A应被淘汰")
	}

	val, _ = n.Get("B")
	if string(val) != "ValB" {
		t.Errorf("Get(B) = %q, 期望 ValB", val)
	}
}

// TestNode_StopAfterWrite 测试写入后停止再启动数据保留
func TestNode_StopAfterWrite(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)
	n.Start()

	n.Set("key1", []byte("value1"))
	n.Stop()

	// 停止后无法操作
	_, err := n.Get("key1")
	if err != node.ErrNodeStopped {
		t.Errorf("停止后 Get 错误 = %v, 期望 ErrNodeStopped", err)
	}

	// 重新启动
	n.Start()

	// 数据应保留（LRU实例未销毁）
	val, err := n.Get("key1")
	if err != nil {
		t.Fatalf("重启后 Get(key1) 失败: %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("重启后 Get(key1) = %q, 期望 value1", val)
	}
}

// TestNode_MultipleStartStop 测试多次启动和停止
func TestNode_MultipleStartStop(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	for i := 0; i < 10; i++ {
		n.Start()
		if n.GetStatus() != node.StatusRunning {
			t.Errorf("第%d次 Start后状态不正确", i+1)
		}

		n.Set(fmt.Sprintf("key_%d", i), []byte(fmt.Sprintf("val_%d", i)))

		n.Stop()
		if n.GetStatus() != node.StatusStopped {
			t.Errorf("第%d次 Stop后状态不正确", i+1)
		}
	}

	// 启动后检查之前写入的数据
	n.Start()
	size := n.Size()
	if size == 0 {
		t.Error("多次启停后应保留数据")
	}
}

// TestNode_GetInfoStopped 测试停止状态下GetInfo仍可用
func TestNode_GetInfoStopped(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	// 即使节点未启动，GetInfo也应可用
	info := n.GetInfo()
	if info["status"] != node.StatusStopped {
		t.Errorf("停止状态 GetInfo().status = %v, 期望 Stopped", info["status"])
	}
}

// TestNode_InitReplacesRing 测试多次Init替换哈希环
func TestNode_InitReplacesRing(t *testing.T) {
	n, _ := node.NewCacheNode("Node1", 100)

	ring1, _ := shard.NewHashRing(50)
	ring2, _ := shard.NewHashRing(100)

	n.Init(ring1)
	if n.GetRing() != ring1 {
		t.Error("第一次Init后应持有ring1")
	}

	n.Init(ring2)
	if n.GetRing() != ring2 {
		t.Error("第二次Init后应持有ring2")
	}
}

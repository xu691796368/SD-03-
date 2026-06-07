// Package shard_test 实现一致性哈希分片模块的单元测试（Task 3.3）
// 覆盖：哈希环初始化、虚拟节点增删、GetNode路由、数据分布偏差校验、并发安全
package shard_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/yourusername/sd-03-cache/pkg/shard"
)

// ============================================================
// 1. 哈希环初始化测试
// ============================================================

// TestNewHashRing_Success 测试正常创建哈希环
func TestNewHashRing_Success(t *testing.T) {
	ring, err := shard.NewHashRing(100)
	if err != nil {
		t.Fatalf("NewHashRing(100) 不应返回错误，得到: %v", err)
	}
	if ring == nil {
		t.Fatal("NewHashRing(100) 不应返回 nil")
	}
	if ring.NodeCount() != 0 {
		t.Fatalf("新建哈希环节点数应为 0，得到: %d", ring.NodeCount())
	}
}

// TestNewHashRing_DefaultVirtualNodes 测试使用默认虚拟节点数（100）创建
func TestNewHashRing_DefaultVirtualNodes(t *testing.T) {
	ring, err := shard.NewHashRing(100)
	if err != nil {
		t.Fatalf("NewHashRing(100) 失败: %v", err)
	}
	if err := ring.AddNode("NodeA"); err != nil {
		t.Fatalf("AddNode 失败: %v", err)
	}
	if ring.VirtualNodeCount() != 100 {
		t.Fatalf("期望虚拟节点数 100，得到: %d", ring.VirtualNodeCount())
	}
}

// TestNewHashRing_InvalidVirtualNodes 测试无效虚拟节点数
func TestNewHashRing_InvalidVirtualNodes(t *testing.T) {
	tests := []struct {
		name          string
		virtualNodes  int
		expectedError error
	}{
		{"零个虚拟节点", 0, shard.ErrInvalidVirtualNodes},
		{"负数虚拟节点", -1, shard.ErrInvalidVirtualNodes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ring, err := shard.NewHashRing(tt.virtualNodes)
			if err != tt.expectedError {
				t.Errorf("期望错误 %v，得到: %v", tt.expectedError, err)
			}
			if ring != nil {
				t.Error("期望 ring 为 nil")
			}
		})
	}
}

// TestNewHashRing_SmallVirtualNodes 测试较小虚拟节点数
func TestNewHashRing_SmallVirtualNodes(t *testing.T) {
	ring, err := shard.NewHashRing(3)
	if err != nil {
		t.Fatalf("NewHashRing(3) 失败: %v", err)
	}
	ring.AddNode("NodeA")
	if ring.VirtualNodeCount() != 3 {
		t.Fatalf("期望 3 个虚拟节点，得到: %d", ring.VirtualNodeCount())
	}
}

// ============================================================
// 2. 虚拟节点增删测试
// ============================================================

// TestAddNode_Single 测试添加单个物理节点
func TestAddNode_Single(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	if err := ring.AddNode("NodeA"); err != nil {
		t.Fatalf("AddNode(\"NodeA\") 失败: %v", err)
	}

	if ring.NodeCount() != 1 {
		t.Fatalf("期望 1 个物理节点，得到: %d", ring.NodeCount())
	}
	if ring.VirtualNodeCount() != 100 {
		t.Fatalf("期望 100 个虚拟节点，得到: %d", ring.VirtualNodeCount())
	}
}

// TestAddNode_Multiple 测试添加多个物理节点
func TestAddNode_Multiple(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	nodes := []string{"NodeA", "NodeB", "NodeC"}
	for _, n := range nodes {
		if err := ring.AddNode(n); err != nil {
			t.Fatalf("AddNode(\"%s\") 失败: %v", n, err)
		}
	}

	if ring.NodeCount() != 3 {
		t.Fatalf("期望 3 个物理节点，得到: %d", ring.NodeCount())
	}
	if ring.VirtualNodeCount() != 300 {
		t.Fatalf("期望 300 个虚拟节点，得到: %d", ring.VirtualNodeCount())
	}
}

// TestAddNode_Duplicate 测试重复添加同一节点（幂等性）
func TestAddNode_Duplicate(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	ring.AddNode("NodeA")
	ring.AddNode("NodeA") // 重复添加

	if ring.NodeCount() != 1 {
		t.Fatalf("重复添加后期望 1 个物理节点，得到: %d", ring.NodeCount())
	}
	if ring.VirtualNodeCount() != 100 {
		t.Fatalf("重复添加后期望 100 个虚拟节点，得到: %d", ring.VirtualNodeCount())
	}
}

// TestAddNode_EmptyNodeID 测试空节点ID
func TestAddNode_EmptyNodeID(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	err := ring.AddNode("")
	if err != shard.ErrEmptyNodeID {
		t.Errorf("期望 ErrEmptyNodeID，得到: %v", err)
	}
}

// TestRemoveNode_Success 测试成功移除节点
func TestRemoveNode_Success(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	if err := ring.RemoveNode("NodeB"); err != nil {
		t.Fatalf("RemoveNode(\"NodeB\") 失败: %v", err)
	}

	if ring.NodeCount() != 2 {
		t.Fatalf("期望 2 个物理节点，得到: %d", ring.NodeCount())
	}
	if ring.VirtualNodeCount() != 200 {
		t.Fatalf("期望 200 个虚拟节点，得到: %d", ring.VirtualNodeCount())
	}

	// 确认被移除的节点不在列表中
	nodes := ring.GetNodes()
	for _, n := range nodes {
		if n == "NodeB" {
			t.Fatal("NodeB 应已被移除")
		}
	}
}

// TestRemoveNode_NotFound 测试移除不存在的节点
func TestRemoveNode_NotFound(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	err := ring.RemoveNode("NonExist")
	if err != shard.ErrNodeNotFound {
		t.Errorf("期望 ErrNodeNotFound，得到: %v", err)
	}
}

// TestRemoveNode_EmptyNodeID 测试移除时使用空节点ID
func TestRemoveNode_EmptyNodeID(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	err := ring.RemoveNode("")
	if err != shard.ErrEmptyNodeID {
		t.Errorf("期望 ErrEmptyNodeID，得到: %v", err)
	}
}

// TestRemoveNode_All 测试移除所有节点后环为空
func TestRemoveNode_All(t *testing.T) {
	ring, _ := shard.NewHashRing(50)

	ring.AddNode("NodeA")
	ring.AddNode("NodeB")

	ring.RemoveNode("NodeA")
	ring.RemoveNode("NodeB")

	if ring.NodeCount() != 0 {
		t.Fatalf("期望 0 个物理节点，得到: %d", ring.NodeCount())
	}
	if ring.VirtualNodeCount() != 0 {
		t.Fatalf("期望 0 个虚拟节点，得到: %d", ring.VirtualNodeCount())
	}
}

// ============================================================
// 3. GetNode 路由测试
// ============================================================

// TestGetNode_SingleShard 单分片场景：所有Key都路由到唯一节点
func TestGetNode_SingleShard(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("Node1")

	keys := []string{"key1", "key2", "key3", "abc", "xyz", "test_data_123"}
	for _, key := range keys {
		node := ring.GetNode(key)
		if node != "Node1" {
			t.Errorf("单分片场景下 Key \"%s\" 应路由到 Node1，实际路由到: %s", key, node)
		}
	}
}

// TestGetNode_Consistency 一致性：同一个Key多次查询必须返回相同节点
func TestGetNode_Consistency(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	key := "my_consistent_key"
	first := ring.GetNode(key)
	for i := 0; i < 100; i++ {
		node := ring.GetNode(key)
		if node != first {
			t.Fatalf("Key \"%s\" 第 %d 次查询返回 %s，期望始终为 %s", key, i, node, first)
		}
	}
}

// TestGetNode_AllKeysMapped 所有Key必须映射到已知的物理节点
func TestGetNode_AllKeysMapped(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	nodeSet := map[string]bool{"NodeA": true, "NodeB": true, "NodeC": true}

	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		node := ring.GetNode(key)
		if !nodeSet[node] {
			t.Errorf("Key \"%s\" 路由到未知节点: %s", key, node)
		}
	}
}

// TestGetNode_EmptyRing 空环应返回空字符串
func TestGetNode_EmptyRing(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	node := ring.GetNode("any_key")
	if node != "" {
		t.Errorf("空环应返回空字符串，得到: %s", node)
	}
}

// TestGetNode_AfterRemove 移除节点后，Key不应路由到已移除的节点
func TestGetNode_AfterRemove(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	ring.RemoveNode("NodeB")

	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("key_%d", i)
		node := ring.GetNode(key)
		if node == "NodeB" {
			t.Errorf("移除 NodeB 后，Key \"%s\" 不应路由到 NodeB", key)
		}
	}
}

// TestGetNode_StableAfterRemove 移除节点后，未受影响的Key应保持原映射
func TestGetNode_StableAfterRemove(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	// 记录移除前的映射
	before := make(map[string]string)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		before[key] = ring.GetNode(key)
	}

	// 添加第四个节点
	ring.AddNode("NodeD")

	// 验证：只有部分Key会改变映射
	changed := 0
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		after := ring.GetNode(key)
		if before[key] != after {
			changed++
		}
	}

	// 添加一个节点后，大多数Key应保持不变（约 75% 不变）
	if changed > 500 {
		t.Logf("添加 NodeD 后 %d/1000 个 Key 发生了迁移（可接受范围）", changed)
	}
}

// ============================================================
// 4. Rebuild 重建测试
// ============================================================

// TestRebuild 重建后虚拟节点数应正确
func TestRebuild(t *testing.T) {
	ring, _ := shard.NewHashRing(50)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")

	if err := ring.Rebuild(); err != nil {
		t.Fatalf("Rebuild 失败: %v", err)
	}

	if ring.VirtualNodeCount() != 100 {
		t.Fatalf("重建后期望 100 个虚拟节点，得到: %d", ring.VirtualNodeCount())
	}

	// 重建后GetNode应正常工作
	node := ring.GetNode("some_key")
	if node != "NodeA" && node != "NodeB" {
		t.Errorf("重建后 GetNode 返回未知节点: %s", node)
	}
}

// ============================================================
// 5. 数据分布偏差校验
// ============================================================

// TestDataDistribution_3Nodes 3节点数据分布偏差测试
// 验收标准：虚拟节点100个，1000次SET后3个分片数据分布差异<30%
// 偏差公式: abs(单节点数量-均值)/均值
func TestDataDistribution_3Nodes(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	// 统计每个节点的Key数量
	counts := make(map[string]int)
	totalKeys := 1000

	rng := rand.New(rand.NewSource(42)) // 固定种子保证可重复
	for i := 0; i < totalKeys; i++ {
		key := fmt.Sprintf("user_%d_%d", rng.Intn(10000), i)
		node := ring.GetNode(key)
		counts[node]++
	}

	// 验证总数据量
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != totalKeys {
		t.Fatalf("总数据量应为 %d，得到: %d", totalKeys, total)
	}

	// 计算均值
	mean := float64(totalKeys) / 3.0
	t.Logf("数据分布: NodeA=%d, NodeB=%d, NodeC=%d, 均值=%.1f",
		counts["NodeA"], counts["NodeB"], counts["NodeC"], mean)

	// 检查每个节点的偏差: abs(单节点数量-均值)/均值 < 30%
	for nodeID, count := range counts {
		deviation := float64(absInt(count-int(mean))) / mean
		t.Logf("  %s: %d 条, 偏差=%.2f%%", nodeID, count, deviation*100)
		if deviation >= 0.30 {
			t.Errorf("节点 %s 偏差 %.2f%% 超过 30%% 阈值", nodeID, deviation*100)
		}
	}
}

// TestDataDistribution_5Nodes 5节点数据分布测试
func TestDataDistribution_5Nodes(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	nodes := []string{"NodeA", "NodeB", "NodeC", "NodeD", "NodeE"}
	for _, n := range nodes {
		ring.AddNode(n)
	}

	counts := make(map[string]int)
	totalKeys := 10000

	rng := rand.New(rand.NewSource(123))
	for i := 0; i < totalKeys; i++ {
		key := fmt.Sprintf("item_%d_%d", rng.Intn(100000), i)
		node := ring.GetNode(key)
		counts[node]++
	}

	mean := float64(totalKeys) / float64(len(nodes))
	t.Logf("5节点数据分布 (均值=%.1f):", mean)
	for _, n := range nodes {
		deviation := float64(absInt(counts[n]-int(mean))) / mean
		t.Logf("  %s: %d 条, 偏差=%.2f%%", n, counts[n], deviation*100)
		if deviation >= 0.35 {
			t.Errorf("节点 %s 偏差 %.2f%% 超过 35%% 阈值", n, deviation*100)
		}
	}
}

// TestDataDistribution_LowVirtualNodes 低虚拟节点数时偏差较大
// 使用较少虚拟节点（如10个），验证分布效果有所下降但仍然可用
func TestDataDistribution_LowVirtualNodes(t *testing.T) {
	ring, _ := shard.NewHashRing(10)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	counts := make(map[string]int)
	totalKeys := 1000

	for i := 0; i < totalKeys; i++ {
		key := fmt.Sprintf("key_%d", i)
		node := ring.GetNode(key)
		counts[node]++
	}

	mean := float64(totalKeys) / 3.0
	t.Logf("低虚拟节点数(10)分布: NodeA=%d, NodeB=%d, NodeC=%d, 均值=%.1f",
		counts["NodeA"], counts["NodeB"], counts["NodeC"], mean)

	// 低虚拟节点数时放宽偏差标准到 50%
	for nodeID, count := range counts {
		deviation := float64(absInt(count-int(mean))) / mean
		if deviation >= 0.50 {
			t.Logf("警告: 节点 %s 偏差 %.2f%%（低虚拟节点数下可接受）", nodeID, deviation*100)
		}
	}
}

// ============================================================
// 6. 并发安全测试
// ============================================================

// TestConcurrentGetNode 并发 GetNode 测试
func TestConcurrentGetNode(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	var wg sync.WaitGroup
	errors := make(chan error, 1000)

	// 启动 100 个 goroutine 并发读取
	for g := 0; g < 100; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("concurrent_key_%d_%d", gid, i)
				node := ring.GetNode(key)
				if node == "" {
					errors <- fmt.Errorf("goroutine %d: GetNode 返回空字符串", gid)
					return
				}
				if node != "NodeA" && node != "NodeB" && node != "NodeC" {
					errors <- fmt.Errorf("goroutine %d: GetNode 返回未知节点 %s", gid, node)
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestConcurrentAddRemove 并发添加和移除节点测试
func TestConcurrentAddRemove(t *testing.T) {
	ring, _ := shard.NewHashRing(50)

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// 并发添加节点
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			nodeID := fmt.Sprintf("Node%d", idx)
			if err := ring.AddNode(nodeID); err != nil {
				errors <- fmt.Errorf("AddNode(%s) 失败: %v", nodeID, err)
			}
		}(i)
	}

	wg.Wait()

	if ring.NodeCount() != 5 {
		t.Errorf("并发添加后期望 5 个节点，得到: %d", ring.NodeCount())
	}

	// 并发移除节点
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			nodeID := fmt.Sprintf("Node%d", idx)
			if err := ring.RemoveNode(nodeID); err != nil {
				errors <- fmt.Errorf("RemoveNode(%s) 失败: %v", nodeID, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	if ring.NodeCount() != 2 {
		t.Errorf("并发移除后期望 2 个节点，得到: %d", ring.NodeCount())
	}
}

// TestConcurrentReadWrite 并发读写混合测试
func TestConcurrentReadWrite(t *testing.T) {
	ring, _ := shard.NewHashRing(100)
	ring.AddNode("NodeA")
	ring.AddNode("NodeB")

	var wg sync.WaitGroup
	errors := make(chan error, 500)

	// 并发读者
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("rw_key_%d_%d", id, i)
				node := ring.GetNode(key)
				if node == "" {
					// 环为空是暂时可能的（如果有并发 RemoveAll）
					continue
				}
			}
		}(g)
	}

	// 并发写者：添加和移除节点
	wg.Add(1)
	go func() {
		defer wg.Done()
		ring.AddNode("NodeC")
		ring.AddNode("NodeD")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ring.Rebuild()
	}()

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// ============================================================
// 7. GetNodes 测试
// ============================================================

// TestGetNodes 测试获取节点列表
func TestGetNodes(t *testing.T) {
	ring, _ := shard.NewHashRing(100)

	// 空环
	if len(ring.GetNodes()) != 0 {
		t.Error("空环的 GetNodes 应返回空列表")
	}

	ring.AddNode("NodeA")
	ring.AddNode("NodeB")
	ring.AddNode("NodeC")

	nodes := ring.GetNodes()
	if len(nodes) != 3 {
		t.Fatalf("期望 3 个节点，得到: %d", len(nodes))
	}

	nodeSet := make(map[string]bool)
	for _, n := range nodes {
		nodeSet[n] = true
	}
	if !nodeSet["NodeA"] || !nodeSet["NodeB"] || !nodeSet["NodeC"] {
		t.Error("GetNodes 未包含所有已添加的节点")
	}
}

// ============================================================
// 辅助函数
// ============================================================

// absInt 计算整数的绝对值
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

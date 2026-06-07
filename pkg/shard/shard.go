// Package shard 实现一致性哈希（Consistent Hashing）分片算法
// 使用 FNV-1a 哈希算法和虚拟节点机制，将 Key 均匀映射到物理节点
package shard

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
	"sort"
	"sync"
)

// ============ 错误定义 ============

var (
	// ErrInvalidVirtualNodes 虚拟节点数参数无效（<=0）
	ErrInvalidVirtualNodes = errors.New("shard: virtualNodes must be positive")

	// ErrEmptyNodeID 节点ID为空
	ErrEmptyNodeID = errors.New("shard: nodeID must not be empty")

	// ErrNodeNotFound 节点不存在
	ErrNodeNotFound = errors.New("shard: node not found")

	// ErrEmptyRing 哈希环为空
	ErrEmptyRing = errors.New("shard: hash ring is empty")
)

// ============ 数据结构 ============

// VirtualNode 虚拟节点结构
// 每个物理节点对应多个虚拟节点，虚拟节点名称格式为 "NodeID#index"
type VirtualNode struct {
	NodeID      string // 物理节点ID
	VirtualName string // 虚拟节点名称（如 "NodeA#1"）
	Hash        uint64 // 虚拟节点在哈希环上的哈希值
}

// HashRing 一致性哈希环
// 维护一个有序的虚拟节点哈希值数组，通过二分查找实现高效路由
type HashRing struct {
	virtualNodes int                // 每个物理节点的虚拟节点数（默认100）
	physicalSet  map[string]struct{} // 物理节点ID集合（用于去重）
	sortedHashes []uint64           // 已排序的虚拟节点哈希值数组
	hashToNode   map[uint64]string  // 哈希值 -> 物理节点ID 映射
	mu           sync.RWMutex       // 读写锁保护并发访问
}

// ============ 构造函数 ============

// NewHashRing 创建一致性哈希环
// virtualNodes: 每个物理节点的虚拟节点数，必须 > 0
func NewHashRing(virtualNodes int) (*HashRing, error) {
	if virtualNodes <= 0 {
		return nil, ErrInvalidVirtualNodes
	}
	return &HashRing{
		virtualNodes: virtualNodes,
		physicalSet:  make(map[string]struct{}),
		sortedHashes: make([]uint64, 0),
		hashToNode:   make(map[uint64]string),
	}, nil
}

// ============ 核心方法 ============

// AddNode 添加物理节点到哈希环
// 为该节点创建 virtualNodes 个虚拟节点，并重新排序哈希环
func (r *HashRing) AddNode(nodeID string) error {
	if nodeID == "" {
		return ErrEmptyNodeID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查节点是否已存在
	if _, exists := r.physicalSet[nodeID]; exists {
		return nil // 幂等：节点已存在，不重复添加
	}

	// 添加物理节点
	r.physicalSet[nodeID] = struct{}{}

	// 为该物理节点创建虚拟节点（使用两阶段 FNV-1a 哈希提升分布均匀性）
	for i := 0; i < r.virtualNodes; i++ {
		hash := virtualNodeHash(nodeID, i)
		r.hashToNode[hash] = nodeID
	}

	// 重新构建有序哈希数组
	r.rebuildSortedHashes()

	return nil
}

// RemoveNode 从哈希环移除物理节点
// 删除该节点所有虚拟节点，并重新排序哈希环
func (r *HashRing) RemoveNode(nodeID string) error {
	if nodeID == "" {
		return ErrEmptyNodeID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查节点是否存在
	if _, exists := r.physicalSet[nodeID]; !exists {
		return ErrNodeNotFound
	}

	// 移除该节点所有虚拟节点的哈希映射
	for i := 0; i < r.virtualNodes; i++ {
		hash := virtualNodeHash(nodeID, i)
		delete(r.hashToNode, hash)
	}

	// 从物理节点集合中移除
	delete(r.physicalSet, nodeID)

	// 重新构建有序哈希数组
	r.rebuildSortedHashes()

	return nil
}

// Rebuild 重新构建哈希环
// 根据当前物理节点集合，重新生成所有虚拟节点并排序
func (r *HashRing) Rebuild() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 清空已有数据
	r.hashToNode = make(map[uint64]string)
	r.sortedHashes = make([]uint64, 0)

	// 为每个物理节点重新创建虚拟节点
	for nodeID := range r.physicalSet {
		for i := 0; i < r.virtualNodes; i++ {
			hash := virtualNodeHash(nodeID, i)
			r.hashToNode[hash] = nodeID
		}
	}

	// 重新排序
	r.rebuildSortedHashes()

	return nil
}

// GetNode 根据Key确定所属的物理节点
// 使用 FNV-1a 计算 Key 的哈希值，在环上顺时针查找最近的虚拟节点
// 若哈希环为空则返回空字符串
func (r *HashRing) GetNode(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.sortedHashes) == 0 {
		return ""
	}

	// 计算 Key 的哈希值
	hash := fnvHash(key)

	// 二分查找：找到第一个 >= hash 的虚拟节点（顺时针方向）
	idx := sort.Search(len(r.sortedHashes), func(i int) bool {
		return r.sortedHashes[i] >= hash
	})

	// 若超出环尾，则回绕到环首
	if idx >= len(r.sortedHashes) {
		idx = 0
	}

	return r.hashToNode[r.sortedHashes[idx]]
}

// GetNodes 获取当前所有物理节点ID列表
func (r *HashRing) GetNodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]string, 0, len(r.physicalSet))
	for nodeID := range r.physicalSet {
		nodes = append(nodes, nodeID)
	}
	return nodes
}

// NodeCount 返回当前物理节点数量
func (r *HashRing) NodeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.physicalSet)
}

// VirtualNodeCount 返回当前虚拟节点总数
func (r *HashRing) VirtualNodeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sortedHashes)
}

// ============ 内部方法 ============

// rebuildSortedHashes 重建有序哈希数组
// 调用方必须持有写锁
func (r *HashRing) rebuildSortedHashes() {
	hashes := make([]uint64, 0, len(r.hashToNode))
	for h := range r.hashToNode {
		hashes = append(hashes, h)
	}
	sort.Slice(hashes, func(i, j int) bool {
		return hashes[i] < hashes[j]
	})
	r.sortedHashes = hashes
}

// ============ 工具函数 ============

// fnvHash 使用 FNV-1a 算法计算字符串的 64 位哈希值
func fnvHash(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}

// virtualNodeHash 使用两阶段 FNV-1a 计算虚拟节点的哈希值
// 第一阶段：对 nodeID 计算 FNV-1a 得到种子值
// 第二阶段：将种子与索引混合后再次 FNV-1a，确保虚拟节点在环上均匀分布
func virtualNodeHash(nodeID string, index int) uint64 {
	// 阶段1：计算 nodeID 的哈希作为种子
	seed := fnvHash(nodeID)

	// 阶段2：将种子与索引进行混合（XOR + 乘以质数），再用 FNV-1a 哈希
	// 乘以 FNV 质数 0x100000001b3 确保相邻索引产生差异巨大的输入
	mixed := seed ^ uint64(index)*0x100000001b3

	h := fnv.New64a()
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], mixed)
	h.Write(buf[:])
	return h.Sum64()
}

// virtualNodeName 生成虚拟节点名称
// 格式: "nodeID#index"（如 "NodeA#0", "NodeA#1", ...）
func virtualNodeName(nodeID string, index int) string {
	return nodeID + "#" + itoa(index)
}

// itoa 将非负整数转换为十进制字符串（仅依赖标准库，避免 fmt 开销）
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := make([]byte, 0, 20)
	for i > 0 {
		digits = append(digits, byte('0'+i%10))
		i /= 10
	}
	// 反转
	for l, r := 0, len(digits)-1; l < r; l, r = l+1, r-1 {
		digits[l], digits[r] = digits[r], digits[l]
	}
	return string(digits)
}

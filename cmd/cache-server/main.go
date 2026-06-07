// Package main 分布式缓存系统主程序入口
//
// 初始化流程：
//  1. 创建 HashRing（一致性哈希环，100个虚拟节点/物理节点）
//  2. 创建 CacheNode（3个缓存节点，每个容量10000条）
//  3. 将节点加入哈希环并初始化
//  4. 启动所有缓存节点
//  5. 设置主从复制关系（Node-1 为主，Node-2、Node-3 为从）
//  6. 创建并启动 TCPServer（监听 :7000）
//  7. 等待 SIGINT/SIGTERM 信号，优雅关闭
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/sd-03-cache/pkg/node"
	"github.com/yourusername/sd-03-cache/pkg/replication"
	"github.com/yourusername/sd-03-cache/pkg/server"
	"github.com/yourusername/sd-03-cache/pkg/shard"
)

// ============ 默认配置常量 ============

const (
	// DefaultAddress 服务器监听地址（默认端口7000）
	DefaultAddress = ":7000"

	// VirtualNodeCount 每个物理节点的虚拟节点数
	VirtualNodeCount = 100

	// NodeCount 缓存节点数量
	NodeCount = 3

	// CacheCapacity 每个缓存节点的LRU容量
	CacheCapacity = 10000
)

// nodeConfigs 缓存节点配置列表
var nodeConfigs = []struct {
	id       string
	capacity int
}{
	{id: "Node-1", capacity: CacheCapacity},
	{id: "Node-2", capacity: CacheCapacity},
	{id: "Node-3", capacity: CacheCapacity},
}

// main 程序入口
func main() {
	log.Println("[Main] SD-03 Distributed Cache Server Starting...")

	// ---------------------------------------------------------------
	// 步骤1：创建一致性哈希环
	// ---------------------------------------------------------------
	ring, err := shard.NewHashRing(VirtualNodeCount)
	if err != nil {
		log.Fatalf("[Main] Failed to create hash ring: %v", err)
	}

	// ---------------------------------------------------------------
	// 步骤2：创建缓存节点
	// ---------------------------------------------------------------
	var nodes []*node.CacheNode
	for _, cfg := range nodeConfigs {
		n, err := node.NewCacheNode(cfg.id, cfg.capacity)
		if err != nil {
			log.Fatalf("[Main] Failed to create node %s: %v", cfg.id, err)
		}

		// 将节点ID加入哈希环
		if err := ring.AddNode(cfg.id); err != nil {
			log.Fatalf("[Main] Failed to add node %s to ring: %v", cfg.id, err)
		}

		// 初始化节点（绑定哈希环）
		if err := n.Init(ring); err != nil {
			log.Fatalf("[Main] Failed to init node %s: %v", cfg.id, err)
		}

		nodes = append(nodes, n)
		log.Printf("[Main] Created node: %s (capacity=%d)", cfg.id, cfg.capacity)
	}

	// ---------------------------------------------------------------
	// 步骤3：启动所有缓存节点
	// ---------------------------------------------------------------
	for _, n := range nodes {
		if err := n.Start(); err != nil {
			log.Fatalf("[Main] Failed to start node %s: %v", n.GetNodeID(), err)
		}
		log.Printf("[Main] Node started: %s", n.GetNodeID())
	}

	// ---------------------------------------------------------------
	// 步骤4：设置主从复制关系（P0简化版：1主1从）
	//   - Node-1 → Master（主节点）
	//   - Node-2 → Slave（从节点）
	//   - Node-3 → Running（普通节点）
	// ---------------------------------------------------------------
	rc, err := replication.NewReplicationController(nodes)
	if err != nil {
		log.Fatalf("[Main] Failed to create replication controller: %v", err)
	}

	if err := rc.SetMasterSlave("Node-1", "Node-2"); err != nil {
		log.Fatalf("[Main] Failed to set master-slave: %v", err)
	}
	log.Println("[Main] Replication configured: Node-1 (Master) → Node-2 (Slave)")

	// ---------------------------------------------------------------
	// 步骤5：创建并启动TCP服务器
	// ---------------------------------------------------------------
	srv, err := server.NewTCPServer(DefaultAddress, nodes, ring)
	if err != nil {
		log.Fatalf("[Main] Failed to create TCP server: %v", err)
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("[Main] Failed to start TCP server: %v", err)
	}
	log.Printf("[Main] TCP server started on %s", DefaultAddress)

	// ---------------------------------------------------------------
	// 步骤6：等待中断信号，优雅关闭
	// ---------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	log.Println("[Main] SD-03 Distributed Cache Server is running. Press Ctrl+C to stop.")

	sig := <-quit
	log.Printf("[Main] Received signal: %v, shutting down...", sig)

	// 停止TCP服务器
	if err := srv.Stop(); err != nil {
		log.Printf("[Main] Error stopping TCP server: %v", err)
	}

	// 停止所有缓存节点
	for _, n := range nodes {
		if err := n.Stop(); err != nil {
			log.Printf("[Main] Error stopping node %s: %v", n.GetNodeID(), err)
		}
	}

	log.Println("[Main] SD-03 Distributed Cache Server stopped gracefully.")
}

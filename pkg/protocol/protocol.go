// Package protocol 实现自定义二进制协议编解码
// 协议格式：Command (1B) + KeyLen (4B) + ValueLen (4B) + Data
// 使用大端字节序（Big-Endian，网络字节序）
package protocol

import (
	"fmt"
)

// ============ 命令码常量 ============

// Command 命令类型
type Command uint8

const (
	// CMD_GET GET命令 - 查询指定Key的值
	CMD_GET Command = 0x01

	// CMD_SET SET命令 - 设置键值对
	CMD_SET Command = 0x02

	// CMD_DELETE DELETE命令 - 删除指定Key
	CMD_DELETE Command = 0x03

	// CMD_INFO INFO命令 - 获取服务器信息
	CMD_INFO Command = 0x04
)

// String 返回命令的字符串表示
func (c Command) String() string {
	switch c {
	case CMD_GET:
		return "GET"
	case CMD_SET:
		return "SET"
	case CMD_DELETE:
		return "DELETE"
	case CMD_INFO:
		return "INFO"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02X)", uint8(c))
	}
}

// ============ 错误码常量 ============

// ErrorCode 错误码类型
type ErrorCode uint8

const (
	// SUCCESS 成功
	SUCCESS ErrorCode = 0x00

	// ERROR_UNKNOWN_COMMAND 未知命令
	ERROR_UNKNOWN_COMMAND ErrorCode = 0x01

	// ERROR_INVALID_KEY 无效的Key
	ERROR_INVALID_KEY ErrorCode = 0x02

	// ERROR_INVALID_VALUE 无效的Value
	ERROR_INVALID_VALUE ErrorCode = 0x03

	// ERROR_CACHE_FULL 缓存已满
	ERROR_CACHE_FULL ErrorCode = 0x04

	// ERROR_FRAME_TOO_SHORT 协议帧过短
	ERROR_FRAME_TOO_SHORT ErrorCode = 0x05

	// ERROR_FRAME_MISMATCH 协议帧长度不匹配
	ERROR_FRAME_MISMATCH ErrorCode = 0x06
)

// String 返回错误码的字符串表示
func (e ErrorCode) String() string {
	switch e {
	case SUCCESS:
		return "SUCCESS"
	case ERROR_UNKNOWN_COMMAND:
		return "ERROR_UNKNOWN_COMMAND"
	case ERROR_INVALID_KEY:
		return "ERROR_INVALID_KEY"
	case ERROR_INVALID_VALUE:
		return "ERROR_INVALID_VALUE"
	case ERROR_CACHE_FULL:
		return "ERROR_CACHE_FULL"
	case ERROR_FRAME_TOO_SHORT:
		return "ERROR_FRAME_TOO_SHORT"
	case ERROR_FRAME_MISMATCH:
		return "ERROR_FRAME_MISMATCH"
	default:
		return fmt.Sprintf("UNKNOWN_ERROR(0x%02X)", uint8(e))
	}
}

// ============ 协议帧结构体 ============

// ProtocolFrame 协议帧结构体
// 帧格式：Command (1B) + KeyLen (4B) + ValueLen (4B) + Key (KeyLen) + Value (ValueLen)
type ProtocolFrame struct {
	Command uint8    // 命令码 (1字节)
	KeyLen  uint32   // Key长度 (4字节)
	ValueLen uint32  // Value长度 (4字节)
	Key     []byte   // Key数据
	Value   []byte   // Value数据
}

// ============ 协议常量定义 ============

const (
	// FrameHeaderSize 协议帧头部长度（Command + KeyLen + ValueLen）
	FrameHeaderSize = 1 + 4 + 4 // 9字节

	// MaxKeyLength 最大Key长度 (1MB)
	MaxKeyLength = 1024 * 1024

	// MaxValueLength 最大Value长度 (1MB)
	MaxValueLength = 1024 * 1024
)

// ============ 帮助函数 ============

// NewFrame 创建新的协议帧
func NewFrame(command uint8, key []byte, value []byte) *ProtocolFrame {
	if key == nil {
		key = []byte{}
	}
	if value == nil {
		value = []byte{}
	}
	return &ProtocolFrame{
		Command: command,
		Key:     key,
		Value:   value,
		KeyLen:  uint32(len(key)),
		ValueLen: uint32(len(value)),
	}
}

// Copy 创建帧的深拷贝
func (f *ProtocolFrame) Copy() *ProtocolFrame {
	if f == nil {
		return nil
	}
	return &ProtocolFrame{
		Command: f.Command,
		KeyLen:  f.KeyLen,
		ValueLen: f.ValueLen,
		Key:     append([]byte{}, f.Key...),
		Value:   append([]byte{}, f.Value...),
	}
}

// FrameSize 计算帧的总大小
func (f *ProtocolFrame) FrameSize() uint32 {
	return FrameHeaderSize + f.KeyLen + f.ValueLen
}

// Equals 判断两个帧是否相等
func (f *ProtocolFrame) Equals(other *ProtocolFrame) bool {
	if f == nil && other == nil {
		return true
	}
	if f == nil || other == nil {
		return false
	}
	return f.Command == other.Command &&
		f.KeyLen == other.KeyLen &&
		f.ValueLen == other.ValueLen &&
		bytesEqual(f.Key, other.Key) &&
		bytesEqual(f.Value, other.Value)
}

// bytesEqual 辅助函数：比较两个字节数组是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ============ 默认包变量 ============

// DefaultKeyLen 默认Key长度
var DefaultKeyLen uint32 = 256

// DefaultValueLen 默认Value长度
var DefaultValueLen uint32 = 1024

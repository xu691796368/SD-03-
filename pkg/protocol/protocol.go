// Package protocol 实现自定义二进制协议编解码
// 协议格式：Command (1B) + KeyLen (4B) + ValueLen (4B) + Data
// 使用大端字节序（Big-Endian，网络字节序）
package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
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

// ============ 辅助函数 ============

// encodeFrame 辅助函数：编码协议帧
func encodeFrame(cmd uint8, key, value []byte) ([]byte, error) {
	return EncodeRequest(cmd, key, value)
}

// ============ 默认包变量 ============

// DefaultKeyLen 默认Key长度
var DefaultKeyLen uint32 = 256

// DefaultValueLen 默认Value长度
var DefaultValueLen uint32 = 1024

// ============ 序列化/反序列化函数 ============

// EncodeRequest 序列化请求
// 返回error: 参数为空、长度超限、序列化失败等错误
func EncodeRequest(cmd uint8, key, value []byte) ([]byte, error) {
	// 检查参数
	if key == nil {
		return nil, fmt.Errorf("key cannot be nil")
	}
	if len(key) > int(MaxKeyLength) {
		return nil, fmt.Errorf("key length %d exceeds max %d", len(key), MaxKeyLength)
	}
	if value == nil {
		value = []byte{}
	}
	if len(value) > int(MaxValueLength) {
		return nil, fmt.Errorf("value length %d exceeds max %d", len(value), MaxValueLength)
	}

	// 创建缓冲区
	buf := new(bytes.Buffer)

	// 写入命令码 (1字节)
	if err := buf.WriteByte(cmd); err != nil {
		return nil, fmt.Errorf("failed to write command: %w", err)
	}

	// 写入Key长度 (4字节，大端字节序)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(key))); err != nil {
		return nil, fmt.Errorf("failed to write key length: %w", err)
	}

	// 写入Value长度 (4字节，大端字节序)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(value))); err != nil {
		return nil, fmt.Errorf("failed to write value length: %w", err)
	}

	// 写入Key数据
	if _, err := buf.Write(key); err != nil {
		return nil, fmt.Errorf("failed to write key data: %w", err)
	}

	// 写入Value数据
	if _, err := buf.Write(value); err != nil {
		return nil, fmt.Errorf("failed to write value data: %w", err)
	}

	return buf.Bytes(), nil
}

// DecodeRequest 反序列化请求
// 返回error: 数据不足、校验失败、解析错误等错误
func DecodeRequest(data []byte) (*ProtocolFrame, error) {
	if data == nil {
		return nil, fmt.Errorf("input data is nil")
	}

	// 检查数据长度是否足够
	if len(data) < int(FrameHeaderSize) {
		return nil, fmt.Errorf("data too short: %d bytes, need at least %d bytes", len(data), FrameHeaderSize)
	}

	// 解析帧头
	cmd := data[0]
	keyLen := binary.BigEndian.Uint32(data[1:5])
	valueLen := binary.BigEndian.Uint32(data[5:9])

	// 检查剩余数据长度
	totalSize := int(FrameHeaderSize) + int(keyLen) + int(valueLen)
	if len(data) < totalSize {
		return nil, fmt.Errorf("data length mismatch: %d bytes, expected %d bytes", len(data), totalSize)
	}

	// 提取Key和Value
	key := data[9 : 9+int(keyLen)]
	value := data[9+int(keyLen) : 9+int(keyLen)+int(valueLen)]

	// 创建帧
	frame := &ProtocolFrame{
		Command: cmd,
		KeyLen:  keyLen,
		ValueLen: valueLen,
		Key:     key,
		Value:   value,
	}

	return frame, nil
}

// EncodeResponse 序列化响应
// 返回error: 序列化失败等错误
func EncodeResponse(cmd uint8, status uint8, value []byte) ([]byte, error) {
	// 检查参数
	if value == nil {
		value = []byte{}
	}
	if len(value) > int(MaxValueLength) {
		return nil, fmt.Errorf("value length %d exceeds max %d", len(value), MaxValueLength)
	}

	// 创建缓冲区
	buf := new(bytes.Buffer)

	// 写入命令码 (1字节)
	if err := buf.WriteByte(cmd); err != nil {
		return nil, fmt.Errorf("failed to write command: %w", err)
	}

	// 写入Key长度 (4字节，大端字节序)
	if err := binary.Write(buf, binary.BigEndian, uint32(0)); err != nil {
		return nil, fmt.Errorf("failed to write key length: %w", err)
	}

	// 写入Value长度 (4字节，大端字节序)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(value))); err != nil {
		return nil, fmt.Errorf("failed to write value length: %w", err)
	}

	// 写入状态码 (1字节)
	if err := buf.WriteByte(status); err != nil {
		return nil, fmt.Errorf("failed to write status: %w", err)
	}

	// 写入Value数据
	if _, err := buf.Write(value); err != nil {
		return nil, fmt.Errorf("failed to write value data: %w", err)
	}

	return buf.Bytes(), nil
}

// DecodeResponse 反序列化响应
// 返回error: 数据不足、校验失败、解析错误等错误
func DecodeResponse(data []byte) (*ProtocolFrame, error) {
	if data == nil {
		return nil, fmt.Errorf("input data is nil")
	}

	// 检查数据长度是否足够
	if len(data) < int(FrameHeaderSize) {
		return nil, fmt.Errorf("data too short: %d bytes, need at least %d bytes", len(data), FrameHeaderSize)
	}

	// 解析帧头
	cmd := data[0]
	keyLen := binary.BigEndian.Uint32(data[1:5])
	valueLen := binary.BigEndian.Uint32(data[5:9])

	// 检查剩余数据长度
	totalSize := int(FrameHeaderSize) + int(keyLen) + int(valueLen)
	if len(data) < totalSize {
		return nil, fmt.Errorf("data length mismatch: %d bytes, expected %d bytes", len(data), totalSize)
	}

	// 提取状态码和Value
	status := data[9]
	value := data[10 : 10+int(valueLen)]

	// 创建响应帧（响应帧的Key为空）
	frame := &ProtocolFrame{
		Command: cmd,
		KeyLen:  keyLen,
		ValueLen: valueLen,
		Key:     []byte{},
		Value:   value,
	}

	// 注意：响应帧没有额外存储status，status在解码后可从其他地方获取
	// 这里我们将status附加到Value中，方便解析
	// 修改Value，将其变成 "status:value" 格式
	frame.Value = append([]byte{status}, value...)

	return frame, nil
}

// ============ 帧校验错误类型 ============

// FrameError 协议帧校验错误，关联已有的 ErrorCode 常量
// 调用方可通过类型断言提取 ErrorCode 进行程序化判断
type FrameError struct {
	Code    ErrorCode
	Message string
}

// Error 实现 error 接口
func (e *FrameError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code.String(), e.Message)
}

// GetErrorCode 从 error 中提取 ErrorCode
// 若错误为 *FrameError 则返回其 Code，否则返回 ERROR_UNKNOWN_COMMAND
func GetErrorCode(err error) ErrorCode {
	if fe, ok := err.(*FrameError); ok {
		return fe.Code
	}
	return ERROR_UNKNOWN_COMMAND
}

// ============ 帧校验函数 ============

// ValidateFrame 验证协议帧有效性
// 校验内容：nil帧检查、命令码合法性、帧总长度、Key/Value长度限制、
// KeyLen/ValueLen与实际数据一致性、命令特定参数要求
// 返回error: 使用 FrameError 包装对应 ErrorCode，便于调用方程序化处理
func ValidateFrame(frame *ProtocolFrame) error {
	// nil帧检查
	if frame == nil {
		return errors.New("frame is nil")
	}

	// 1. 检查命令码是否合法
	cmd := Command(frame.Command)
	switch cmd {
	case CMD_GET, CMD_SET, CMD_DELETE, CMD_INFO:
		// 合法命令
	default:
		return &FrameError{
			Code:    ERROR_UNKNOWN_COMMAND,
			Message: fmt.Sprintf("invalid command: 0x%02X", frame.Command),
		}
	}

	// 2. 检查帧总长度是否小于帧头大小
	totalSize := frame.FrameSize()
	if totalSize < uint32(FrameHeaderSize) {
		return &FrameError{
			Code:    ERROR_FRAME_TOO_SHORT,
			Message: fmt.Sprintf("frame total size %d is less than header size %d", totalSize, FrameHeaderSize),
		}
	}

	// 3. 检查Key长度是否超过限制
	if frame.KeyLen > MaxKeyLength {
		return &FrameError{
			Code:    ERROR_INVALID_KEY,
			Message: fmt.Sprintf("key length %d exceeds max %d", frame.KeyLen, MaxKeyLength),
		}
	}

	// 4. 检查Value长度是否超过限制
	if frame.ValueLen > MaxValueLength {
		return &FrameError{
			Code:    ERROR_INVALID_VALUE,
			Message: fmt.Sprintf("value length %d exceeds max %d", frame.ValueLen, MaxValueLength),
		}
	}

	// 5. 检查KeyLen与实际Key数据长度一致性
	if len(frame.Key) != int(frame.KeyLen) {
		return &FrameError{
			Code:    ERROR_FRAME_MISMATCH,
			Message: fmt.Sprintf("key length mismatch: header declares %d, actual data is %d", frame.KeyLen, len(frame.Key)),
		}
	}

	// 6. 检查ValueLen与实际Value数据长度一致性
	if len(frame.Value) != int(frame.ValueLen) {
		return &FrameError{
			Code:    ERROR_FRAME_MISMATCH,
			Message: fmt.Sprintf("value length mismatch: header declares %d, actual data is %d", frame.ValueLen, len(frame.Value)),
		}
	}

	// 7. 命令特定校验
	switch cmd {
	case CMD_GET, CMD_DELETE:
		// GET/DELETE命令需要非空Key
		if frame.KeyLen == 0 || len(frame.Key) == 0 {
			return &FrameError{
				Code:    ERROR_INVALID_KEY,
				Message: fmt.Sprintf("%s command requires a non-empty key", cmd.String()),
			}
		}
	case CMD_SET:
		// SET命令需要非空Key
		if frame.KeyLen == 0 || len(frame.Key) == 0 {
			return &FrameError{
				Code:    ERROR_INVALID_KEY,
				Message: "SET command requires a non-empty key",
			}
		}
	case CMD_INFO:
		// INFO命令无额外参数要求
	}

	return nil
}

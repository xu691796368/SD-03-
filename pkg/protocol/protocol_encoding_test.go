// Package protocol tests for Task 1.5 serialization/deserialization
package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestEncodeRequest 测试 EncodeRequest（Task 1.5）
func TestEncodeRequest(t *testing.T) {
	tests := []struct {
		name    string
		cmd     uint8
		key     []byte
		value   []byte
		wantErr bool
		errMsg  string
	}{
		{
			name:    "normal GET request",
			cmd:     uint8(CMD_GET),
			key:     []byte("test_key"),
			value:   []byte("test_value"),
			wantErr: false,
		},
		{
			name:    "normal SET request",
			cmd:     uint8(CMD_SET),
			key:     []byte("user:1"),
			value:   []byte("John Doe"),
			wantErr: false,
		},
		{
			name:    "empty key and value",
			cmd:     uint8(CMD_SET),
			key:     []byte{},
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "nil key",
			cmd:     uint8(CMD_GET),
			key:     nil,
			value:   []byte("value"),
			wantErr: true,
			errMsg:  "key cannot be nil",
		},
		{
			name:    "max length key",
			cmd:     uint8(CMD_SET),
			key:     bytes.Repeat([]byte{'a'}, int(MaxKeyLength)),
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "exceeding max length key",
			cmd:     uint8(CMD_SET),
			key:     bytes.Repeat([]byte{'a'}, int(MaxKeyLength)+1),
			value:   []byte{},
			wantErr: true,
			errMsg:  "key length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := EncodeRequest(tt.cmd, tt.key, tt.value)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if data == nil {
				t.Error("expected non-nil data")
				return
			}

			// 验证数据长度
			expectedLen := FrameHeaderSize + len(tt.key) + len(tt.value)
			if len(data) != expectedLen {
				t.Errorf("encoded data length = %d, want %d", len(data), expectedLen)
			}

			// 验证大端字节序
			if len(tt.key) > 0 {
				// 提取 keyLen（第1-5字节，大端字节序）
				keyLenFromData := binary.BigEndian.Uint32(data[1:5])
				if keyLenFromData != uint32(len(tt.key)) {
					t.Errorf("keyLen from data = %d, want %d", keyLenFromData, len(tt.key))
				}
			}
		})
	}
}

// TestDecodeRequest 测试 DecodeRequest（Task 1.5）
func TestDecodeRequest(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		cmd     uint8
		key     []byte
		value   []byte
		wantErr bool
		errMsg  string
	}{
		{
			name:    "normal GET request",
			data:   createTestFrame(uint8(CMD_GET), []byte("test_key"), []byte("test_value")),
			cmd:     uint8(CMD_GET),
			key:     []byte("test_key"),
			value:   []byte("test_value"),
			wantErr: false,
		},
		{
			name:    "normal SET request",
			data:   createTestFrame(uint8(CMD_SET), []byte("user:1"), []byte("John Doe")),
			cmd:     uint8(CMD_SET),
			key:     []byte("user:1"),
			value:   []byte("John Doe"),
			wantErr: false,
		},
		{
			name:    "empty key and value",
			data:   createTestFrame(uint8(CMD_SET), []byte{}, []byte{}),
			cmd:     uint8(CMD_SET),
			key:     []byte{},
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "too short data",
			data:    []byte{0x01}, // 只有命令码
			wantErr: true,
			errMsg:  "data too short",
		},
		{
			name:    "data length mismatch",
			data:   createTestFrame(uint8(CMD_GET), []byte("key"), []byte("value"))[:10], // 截断数据
			wantErr: true,
			errMsg:  "data length mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := DecodeRequest(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if frame == nil {
				t.Error("expected non-nil frame")
				return
			}

			// 验证解码后的帧
			if frame.Command != tt.cmd {
				t.Errorf("Command = %d, want %d", frame.Command, tt.cmd)
			}
			if string(frame.Key) != string(tt.key) {
				t.Errorf("Key = %s, want %s", frame.Key, tt.key)
			}
			if string(frame.Value) != string(tt.value) {
				t.Errorf("Value = %s, want %s", frame.Value, tt.value)
			}

			// 验证帧头正确解析
			if frame.KeyLen != uint32(len(tt.key)) {
				t.Errorf("keyLen = %d, want %d", frame.KeyLen, len(tt.key))
			}
		})
	}
}

// TestEncodeResponse 测试 EncodeResponse（Task 1.5）
func TestEncodeResponse(t *testing.T) {
	tests := []struct {
		name    string
		cmd     uint8
		status  uint8
		value   []byte
		wantErr bool
		errMsg  string
	}{
		{
			name:    "success response",
			cmd:     uint8(CMD_GET),
			status:  uint8(SUCCESS),
			value:   []byte("test_value"),
			wantErr: false,
		},
		{
			name:    "error response",
			cmd:     uint8(CMD_SET),
			status:  uint8(ERROR_INVALID_KEY),
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "empty value",
			cmd:     uint8(CMD_INFO),
			status:  uint8(SUCCESS),
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "exceeding max length value",
			cmd:     uint8(CMD_SET),
			status:  uint8(SUCCESS),
			value:   bytes.Repeat([]byte{'a'}, int(MaxValueLength)+1),
			wantErr: true,
			errMsg:  "value length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := EncodeResponse(tt.cmd, tt.status, tt.value)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if data == nil {
				t.Error("expected non-nil data")
				return
			}

			// 验证数据长度（响应帧没有Key数据）
			expectedLen := FrameHeaderSize + 0 + len(tt.value) + 1 // +1 for status
			if len(data) != expectedLen {
				t.Errorf("encoded data length = %d, want %d", len(data), expectedLen)
			}

			// 验证大端字节序
			keyLenFromData := binary.BigEndian.Uint32(data[1:5])
			if keyLenFromData != 0 {
				t.Errorf("keyLen from data = %d, want 0", keyLenFromData)
			}

			valueLenFromData := binary.BigEndian.Uint32(data[5:9])
			if valueLenFromData != uint32(len(tt.value)) {
				t.Errorf("valueLen from data = %d, want %d", valueLenFromData, len(tt.value))
			}
		})
	}
}

// TestDecodeResponse 测试 DecodeResponse（Task 1.5）
func TestDecodeResponse(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		cmd     uint8
		status  uint8
		value   []byte
		wantErr bool
		errMsg  string
	}{
		{
			name:    "success GET response",
			data:    createTestResponseFrame(uint8(CMD_GET), uint8(SUCCESS), []byte("test_value")),
			cmd:     uint8(CMD_GET),
			status:  uint8(SUCCESS),
			value:   []byte("test_value"),
			wantErr: false,
		},
		{
			name:    "error SET response",
			data:    createTestResponseFrame(uint8(CMD_SET), uint8(ERROR_INVALID_KEY), []byte{}),
			cmd:     uint8(CMD_SET),
			status:  uint8(ERROR_INVALID_KEY),
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "too short data",
			data:    []byte{0x01}, // 只有命令码
			wantErr: true,
			errMsg:  "data too short",
		},
		{
			name:    "data length mismatch",
			data:    createTestResponseFrame(uint8(CMD_GET), uint8(SUCCESS), []byte("value"))[:10], // 截断数据
			wantErr: true,
			errMsg:  "data length mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := DecodeResponse(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if frame == nil {
				t.Error("expected non-nil frame")
				return
			}

			// 验证解码后的帧（响应帧没有Key）
			if frame.Command != tt.cmd {
				t.Errorf("Command = %d, want %d", frame.Command, tt.cmd)
			}
			if frame.KeyLen != 0 {
				t.Errorf("KeyLen = %d, want 0", frame.KeyLen)
			}
			if len(frame.Value) < 1 {
				t.Errorf("Value length too short, expected at least 1 byte (status)")
			}

			// 验证状态码
			if frame.Value[0] != tt.status {
				t.Errorf("Status = %d, want %d", frame.Value[0], tt.status)
			}

			// 验证Value内容（扣除状态码）
			actualValue := frame.Value[1:]
			if string(actualValue) != string(tt.value) {
				t.Errorf("Value = %s, want %s", actualValue, tt.value)
			}
		})
	}
}

// TestValidateFrame 测试 ValidateFrame（Task 1.5）
func TestValidateFrame(t *testing.T) {
	tests := []struct {
		name    string
		frame   *ProtocolFrame
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid GET frame",
			frame: NewFrame(uint8(CMD_GET), []byte("test_key"), []byte("test_value")),
			wantErr: false,
		},
		{
			name: "valid SET frame",
			frame: NewFrame(uint8(CMD_SET), []byte("user:1"), []byte("John Doe")),
			wantErr: false,
		},
		{
			name: "invalid command",
			frame: NewFrame(0xFF, []byte("test_key"), []byte("test_value")),
			wantErr: true,
			errMsg:  "invalid command",
		},
		{
			name: "exceeding max key length",
			frame: NewFrame(uint8(CMD_GET), bytes.Repeat([]byte{'a'}, int(MaxKeyLength)+1), []byte("test_value")),
			wantErr: true,
			errMsg:  "exceeds max",
		},
		{
			name: "exceeding max value length",
			frame: NewFrame(uint8(CMD_GET), []byte("test_key"), bytes.Repeat([]byte{'a'}, int(MaxValueLength)+1)),
			wantErr: true,
			errMsg:  "exceeds max",
		},
		{
			name: "nil frame",
			frame: nil,
			wantErr: true,
			errMsg:  "frame is nil",
		},
		{
			name: "key length mismatch",
			frame: &ProtocolFrame{
				Command: uint8(CMD_GET),
				KeyLen:  10,
				ValueLen: 5,
				Key:     []byte("short_key"),
				Value:   []byte("value"),
			},
			wantErr: true,
			errMsg:  "key length mismatch",
		},
		{
			name: "value length mismatch",
			frame: &ProtocolFrame{
				Command: uint8(CMD_GET),
				KeyLen:  5,
				ValueLen: 10,
				Key:     []byte("key"),
				Value:   []byte("short_value"),
			},
			wantErr: true,
			errMsg:  "mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFrame(tt.frame)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestRoundTripRequest 测试请求序列化/反序列化往返（Task 1.5）
func TestRoundTripRequest(t *testing.T) {
	testCases := []struct {
		name     string
		cmd      uint8
		key      string
		value    string
	}{
		{"GET", uint8(CMD_GET), "user:123", "John"},
		{"SET", uint8(CMD_SET), "product:456", "iPhone 15"},
		{"DELETE", uint8(CMD_DELETE), "temp:123", ""},
		{"INFO", uint8(CMD_INFO), "", "server_info"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 序列化
			encoded, err := EncodeRequest(tc.cmd, []byte(tc.key), []byte(tc.value))
			if err != nil {
				t.Fatalf("EncodeRequest failed: %v", err)
			}

			// 验证编码长度
			expectedLen := FrameHeaderSize + len(tc.key) + len(tc.value)
			if len(encoded) != expectedLen {
				t.Errorf("encoded length = %d, want %d", len(encoded), expectedLen)
			}

			// 反序列化
			decoded, err := DecodeRequest(encoded)
			if err != nil {
				t.Fatalf("DecodeRequest failed: %v", err)
			}

			// 验证命令码
			if decoded.Command != tc.cmd {
				t.Errorf("Command = %d, want %d", decoded.Command, tc.cmd)
			}

			// 验证Key
			if string(decoded.Key) != tc.key {
				t.Errorf("Key = %s, want %s", decoded.Key, tc.key)
			}

			// 验证Value
			if string(decoded.Value) != tc.value {
				t.Errorf("Value = %s, want %s", decoded.Value, tc.value)
			}

			// 验证长度一致性
			if decoded.KeyLen != uint32(len(tc.key)) {
				t.Errorf("KeyLen = %d, want %d", decoded.KeyLen, len(tc.key))
			}
			if decoded.ValueLen != uint32(len(tc.value)) {
				t.Errorf("ValueLen = %d, want %d", decoded.ValueLen, len(tc.value))
			}
		})
	}
}

// TestRoundTripResponse 测试响应序列化/反序列化往返（Task 1.5）
func TestRoundTripResponse(t *testing.T) {
	testCases := []struct {
		name    string
		cmd     uint8
		status  ErrorCode
		value   string
	}{
		{"success GET", uint8(CMD_GET), SUCCESS, "value123"},
		{"error SET", uint8(CMD_SET), ERROR_INVALID_KEY, "error_msg"},
		{"success DELETE", uint8(CMD_DELETE), SUCCESS, "deleted"},
		{"success INFO", uint8(CMD_INFO), SUCCESS, "info"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 序列化
			encoded, err := EncodeResponse(tc.cmd, uint8(tc.status), []byte(tc.value))
			if err != nil {
				t.Fatalf("EncodeResponse failed: %v", err)
			}

			// 验证编码长度（响应帧没有Key）
			expectedLen := FrameHeaderSize + 0 + len(tc.value) + 1
			if len(encoded) != expectedLen {
				t.Errorf("encoded length = %d, want %d", len(encoded), expectedLen)
			}

			// 反序列化
			decoded, err := DecodeResponse(encoded)
			if err != nil {
				t.Fatalf("DecodeResponse failed: %v", err)
			}

			// 验证命令码
			if decoded.Command != tc.cmd {
				t.Errorf("Command = %d, want %d", decoded.Command, tc.cmd)
			}

			// 验证Key为空
			if decoded.KeyLen != 0 {
				t.Errorf("KeyLen = %d, want 0", decoded.KeyLen)
			}

			// 验证Value
			if len(decoded.Value) < 1 {
				t.Errorf("Value too short, expected at least 1 byte (status)")
			}

			// 验证Value包含状态码（在Value的第0个字节）
			statusFromValue := decoded.Value[0]
			if uint8(statusFromValue) != uint8(tc.status) {
				t.Errorf("Status from value = %d, want %d", statusFromValue, tc.status)
			}
		})
	}
}

// TestBigEndian 测试大端字节序（Task 1.5）
func TestBigEndian(t *testing.T) {
	// 测试大端字节序的 keyLen (4字节)
	data := make([]byte, FrameHeaderSize)
	keyLen := uint32(0x12345678)

	binary.BigEndian.PutUint32(data[1:5], keyLen)

	if binary.BigEndian.Uint32(data[1:5]) != keyLen {
		t.Error("Big-endian encoding failed for keyLen")
	}

	// 测试大端字节序的 valueLen (4字节)
	valueLen := uint32(0x87654321)

	binary.BigEndian.PutUint32(data[5:9], valueLen)

	if binary.BigEndian.Uint32(data[5:9]) != valueLen {
		t.Error("Big-endian encoding failed for valueLen")
	}
}

// TestEdgeCases 测试边界情况（Task 1.5）
func TestEdgeCases(t *testing.T) {
	// 测试最小键和值
	encoded, err := EncodeRequest(uint8(CMD_GET), []byte{}, []byte{})
	if err != nil {
		t.Fatalf("EncodeRequest with empty key and value failed: %v", err)
	}
	decoded, err := DecodeRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeRequest failed: %v", err)
	}
	if decoded.KeyLen != 0 || decoded.ValueLen != 0 {
		t.Error("Empty key and value not decoded correctly")
	}

	// 测试单个字符的键和值
	encoded, err = EncodeRequest(uint8(CMD_SET), []byte{'a'}, []byte{'b'})
	if err != nil {
		t.Fatalf("EncodeRequest with single char failed: %v", err)
	}
	decoded, err = DecodeRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeRequest failed: %v", err)
	}
	if string(decoded.Key) != "a" || string(decoded.Value) != "b" {
		t.Error("Single char key and value not decoded correctly")
	}

	// 测试包含特殊字符的键和值
	specialKey := "key:with:colon"
	specialValue := "value:with:colon"
	encoded, err = EncodeRequest(uint8(CMD_SET), []byte(specialKey), []byte(specialValue))
	if err != nil {
		t.Fatalf("EncodeRequest with special chars failed: %v", err)
	}
	decoded, err = DecodeRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeRequest failed: %v", err)
	}
	if string(decoded.Key) != specialKey || string(decoded.Value) != specialValue {
		t.Error("Special character key and value not decoded correctly")
	}
}

// createTestFrame 创建测试用的协议帧（内部使用）
func createTestFrame(cmd uint8, key, value []byte) []byte {
	data := make([]byte, FrameHeaderSize+len(key)+len(value))
	data[0] = cmd
	binary.BigEndian.PutUint32(data[1:5], uint32(len(key)))
	binary.BigEndian.PutUint32(data[5:9], uint32(len(value)))
	copy(data[9:], key)
	copy(data[9+uint32(len(key)):], value)
	return data
}

// createTestResponseFrame 创建测试用的响应帧（内部使用）
func createTestResponseFrame(cmd uint8, status uint8, value []byte) []byte {
	data := make([]byte, FrameHeaderSize+1+len(value))
	data[0] = cmd
	binary.BigEndian.PutUint32(data[1:5], 0) // Key length is 0 for response
	binary.BigEndian.PutUint32(data[5:9], uint32(len(value)))
	data[9] = status // Status at position 9
	copy(data[10:], value)
	return data
}

// contains 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (containsAll(s, substr)))
}

// containsAll 辅助函数：检查字符串是否包含所有字符
func containsAll(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

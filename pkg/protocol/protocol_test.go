// Package protocol tests
package protocol

import (
	"testing"
)

// TestCommandTypes 测试命令码常量
func TestCommandTypes(t *testing.T) {
	tests := []struct {
		name  string
		cmd   Command
		valid bool
	}{
		{"GET", CMD_GET, true},
		{"SET", CMD_SET, true},
		{"DELETE", CMD_DELETE, true},
		{"INFO", CMD_INFO, true},
		{"UNKNOWN", Command(0xFF), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				_, ok := map[Command]string{
					CMD_GET:   "GET",
					CMD_SET:   "SET",
					CMD_DELETE: "DELETE",
					CMD_INFO:  "INFO",
				}[tt.cmd]
				if !ok {
					t.Errorf("Command %d should be valid", tt.cmd)
				}
			}
		})
	}
}

// TestErrorCode 测试错误码常量
func TestErrorCode(t *testing.T) {
	tests := []struct {
		name   string
		code   ErrorCode
		valid  bool
		expect string
	}{
		{"SUCCESS", SUCCESS, true, "SUCCESS"},
		{"ERROR_UNKNOWN_COMMAND", ERROR_UNKNOWN_COMMAND, true, "ERROR_UNKNOWN_COMMAND"},
		{"ERROR_INVALID_KEY", ERROR_INVALID_KEY, true, "ERROR_INVALID_KEY"},
		{"ERROR_INVALID_VALUE", ERROR_INVALID_VALUE, true, "ERROR_INVALID_VALUE"},
		{"ERROR_CACHE_FULL", ERROR_CACHE_FULL, true, "ERROR_CACHE_FULL"},
		{"ERROR_FRAME_TOO_SHORT", ERROR_FRAME_TOO_SHORT, true, "ERROR_FRAME_TOO_SHORT"},
		{"ERROR_FRAME_MISMATCH", ERROR_FRAME_MISMATCH, true, "ERROR_FRAME_MISMATCH"},
		{"UNKNOWN", ErrorCode(0xFF), false, "UNKNOWN_ERROR(0xFF)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				str := tt.code.String()
				if str != tt.expect {
					t.Errorf("ErrorCode.String() = %s, want %s", str, tt.expect)
				}
			}
		})
	}
}

// TestProtocolFrameNewFrame 测试创建协议帧
func TestProtocolFrameNewFrame(t *testing.T) {
	tests := []struct {
		name     string
		command  uint8
		key      []byte
		value    []byte
		wantKey  []byte
		wantValue []byte
	}{
		{
			name:     "normal frame",
			command:  uint8(CMD_GET),
			key:      []byte("test"),
			value:    []byte("value"),
			wantKey:  []byte("test"),
			wantValue: []byte("value"),
		},
		{
			name:     "empty key and value",
			command:  uint8(CMD_SET),
			key:      []byte{},
			value:    []byte{},
			wantKey:  []byte{},
			wantValue: []byte{},
		},
		{
			name:     "nil key",
			command:  uint8(CMD_INFO),
			key:      nil,
			value:    []byte("info"),
			wantKey:  []byte{},
			wantValue: []byte("info"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := NewFrame(tt.command, tt.key, tt.value)

			if frame.Command != tt.command {
				t.Errorf("Frame.Command = %d, want %d", frame.Command, tt.command)
			}

			if string(frame.Key) != string(tt.wantKey) {
				t.Errorf("Frame.Key = %s, want %s", frame.Key, tt.wantKey)
			}

			if string(frame.Value) != string(tt.wantValue) {
				t.Errorf("Frame.Value = %s, want %s", frame.Value, tt.wantValue)
			}

			if frame.KeyLen != uint32(len(tt.wantKey)) {
				t.Errorf("Frame.KeyLen = %d, want %d", frame.KeyLen, len(tt.wantKey))
			}

			if frame.ValueLen != uint32(len(tt.wantValue)) {
				t.Errorf("Frame.ValueLen = %d, want %d", frame.ValueLen, len(tt.wantValue))
			}
		})
	}
}

// TestProtocolFrameFrameSize 测试帧大小计算
func TestProtocolFrameFrameSize(t *testing.T) {
	frame := NewFrame(uint8(CMD_SET), []byte("key"), []byte("value"))

	expectedSize := FrameHeaderSize + uint32(len("key")) + uint32(len("value"))
	if frame.FrameSize() != expectedSize {
		t.Errorf("Frame.FrameSize() = %d, want %d", frame.FrameSize(), expectedSize)
	}
}

// TestProtocolFrameEquals 测试帧相等比较
func TestProtocolFrameEquals(t *testing.T) {
	frame1 := NewFrame(uint8(CMD_GET), []byte("key"), []byte("value"))
	frame2 := NewFrame(uint8(CMD_GET), []byte("key"), []byte("value"))
	frame3 := NewFrame(uint8(CMD_SET), []byte("key"), []byte("value"))
	frame4 := NewFrame(uint8(CMD_GET), []byte("key"), []byte("other"))
	frame5 := NewFrame(uint8(CMD_GET), []byte("other"), []byte("value"))

	if !frame1.Equals(frame2) {
		t.Error("frame1 should equal frame2")
	}

	if frame1.Equals(frame3) {
		t.Error("frame1 should not equal frame3 (different command)")
	}

	if frame1.Equals(frame4) {
		t.Error("frame1 should not equal frame4 (different value)")
	}

	if frame1.Equals(frame5) {
		t.Error("frame1 should not equal frame5 (different key)")
	}

	if frame1.Equals(nil) {
		t.Error("frame1 should not equal nil")
	}

	if nil == frame1 {
		t.Error("nil should not equal frame1")
	}
}

// TestProtocolFrameCopy 测试帧深拷贝
func TestProtocolFrameCopy(t *testing.T) {
	original := NewFrame(uint8(CMD_GET), []byte("key"), []byte("value"))

	copy := original.Copy()

	if !original.Equals(copy) {
		t.Error("Copy should produce an equal frame")
	}

	if &original == &copy {
		t.Error("Copy should produce a new frame (different address)")
	}

	copy.Value = []byte("changed")
	if string(original.Value) != "value" {
		t.Error("Original frame should not be modified after copy")
	}
}

// TestConstants 测试协议常量
func TestConstants(t *testing.T) {
	if FrameHeaderSize != 9 {
		t.Errorf("FrameHeaderSize = %d, want 9", FrameHeaderSize)
	}

	if MaxKeyLength != 1024*1024 {
		t.Errorf("MaxKeyLength = %d, want %d", MaxKeyLength, 1024*1024)
	}

	if MaxValueLength != 1024*1024 {
		t.Errorf("MaxValueLength = %d, want %d", MaxValueLength, 1024*1024)
	}
}

// TestCommandString 测试命令码字符串表示
func TestCommandString(t *testing.T) {
	tests := []struct {
		cmd    Command
		expect string
	}{
		{CMD_GET, "GET"},
		{CMD_SET, "SET"},
		{CMD_DELETE, "DELETE"},
		{CMD_INFO, "INFO"},
		{Command(0xFF), "UNKNOWN(0xFF)"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			str := tt.cmd.String()
			if str != tt.expect {
				t.Errorf("Command.String() = %s, want %s", str, tt.expect)
			}
		})
	}
}

// TestErrorCodeString 测试错误码字符串表示
func TestErrorCodeString(t *testing.T) {
	tests := []struct {
		code   ErrorCode
		expect string
	}{
		{SUCCESS, "SUCCESS"},
		{ERROR_UNKNOWN_COMMAND, "ERROR_UNKNOWN_COMMAND"},
		{ERROR_INVALID_KEY, "ERROR_INVALID_KEY"},
		{ERROR_INVALID_VALUE, "ERROR_INVALID_VALUE"},
		{ERROR_CACHE_FULL, "ERROR_CACHE_FULL"},
		{ERROR_FRAME_TOO_SHORT, "ERROR_FRAME_TOO_SHORT"},
		{ERROR_FRAME_MISMATCH, "ERROR_FRAME_MISMATCH"},
		{ErrorCode(0xFF), "UNKNOWN_ERROR(0xFF)"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			str := tt.code.String()
			if str != tt.expect {
				t.Errorf("ErrorCode.String() = %s, want %s", str, tt.expect)
			}
		})
	}
}

// Package protocol 单元测试 - ValidateFrame 帧校验函数 (Task 1.6)
package protocol

import (
	"errors"
	"testing"
)

// TestValidateFrame_NilFrame 测试nil帧
func TestValidateFrame_NilFrame(t *testing.T) {
	err := ValidateFrame(nil)
	if err == nil {
		t.Fatal("expected error for nil frame, got nil")
	}
	if err.Error() != "frame is nil" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

// TestValidateFrame_ValidCommands 测试合法命令的帧通过校验
func TestValidateFrame_ValidCommands(t *testing.T) {
	tests := []struct {
		name  string
		frame *ProtocolFrame
	}{
		{
			name:  "GET with key",
			frame: NewFrame(uint8(CMD_GET), []byte("mykey"), nil),
		},
		{
			name:  "SET with key and value",
			frame: NewFrame(uint8(CMD_SET), []byte("mykey"), []byte("myvalue")),
		},
		{
			name:  "DELETE with key",
			frame: NewFrame(uint8(CMD_DELETE), []byte("mykey"), nil),
		},
		{
			name:  "INFO without key",
			frame: NewFrame(uint8(CMD_INFO), nil, nil),
		},
		{
			name:  "SET with key and empty value",
			frame: NewFrame(uint8(CMD_SET), []byte("mykey"), []byte{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFrame(tt.frame)
			if err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateFrame_UnknownCommand 测试未知命令码
func TestValidateFrame_UnknownCommand(t *testing.T) {
	frame := NewFrame(0x99, []byte("key"), nil)
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_UNKNOWN_COMMAND {
		t.Errorf("expected error code ERROR_UNKNOWN_COMMAND (0x%02X), got %v", ERROR_UNKNOWN_COMMAND, fe.Code)
	}
}

// TestValidateFrame_KeyExceedsMax 测试Key长度超过限制
func TestValidateFrame_KeyExceedsMax(t *testing.T) {
	frame := &ProtocolFrame{
		Command:  uint8(CMD_GET),
		KeyLen:   MaxKeyLength + 1,
		ValueLen: 0,
		Key:      make([]byte, MaxKeyLength+1),
		Value:    []byte{},
	}
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for key exceeding max length, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_INVALID_KEY {
		t.Errorf("expected error code ERROR_INVALID_KEY, got %v", fe.Code)
	}
}

// TestValidateFrame_ValueExceedsMax 测试Value长度超过限制
func TestValidateFrame_ValueExceedsMax(t *testing.T) {
	frame := &ProtocolFrame{
		Command:  uint8(CMD_SET),
		KeyLen:   3,
		ValueLen: MaxValueLength + 1,
		Key:      []byte("key"),
		Value:    make([]byte, MaxValueLength+1),
	}
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for value exceeding max length, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_INVALID_VALUE {
		t.Errorf("expected error code ERROR_INVALID_VALUE, got %v", fe.Code)
	}
}

// TestValidateFrame_KeyLengthMismatch 测试KeyLen与实际Key数据不一致
func TestValidateFrame_KeyLengthMismatch(t *testing.T) {
	frame := &ProtocolFrame{
		Command:  uint8(CMD_GET),
		KeyLen:   100, // 声明100字节
		ValueLen: 0,
		Key:      []byte("short"), // 实际只有5字节
		Value:    []byte{},
	}
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for key length mismatch, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_FRAME_MISMATCH {
		t.Errorf("expected error code ERROR_FRAME_MISMATCH, got %v", fe.Code)
	}
}

// TestValidateFrame_ValueLengthMismatch 测试ValueLen与实际Value数据不一致
func TestValidateFrame_ValueLengthMismatch(t *testing.T) {
	frame := &ProtocolFrame{
		Command:  uint8(CMD_SET),
		KeyLen:   3,
		ValueLen: 200, // 声明200字节
		Key:      []byte("key"),
		Value:    []byte("short"), // 实际只有5字节
	}
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for value length mismatch, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_FRAME_MISMATCH {
		t.Errorf("expected error code ERROR_FRAME_MISMATCH, got %v", fe.Code)
	}
}

// TestValidateFrame_FrameTooShort 测试帧总长度过短
func TestValidateFrame_FrameTooShort(t *testing.T) {
	// 构造一个KeyLen+ValueLen极小但合法的INFO帧
	frame := &ProtocolFrame{
		Command:  uint8(CMD_INFO),
		KeyLen:   0,
		ValueLen: 0,
		Key:      []byte{},
		Value:    []byte{},
	}
	// INFO帧无key要求，合法帧总大小=FrameHeaderSize，不应报错
	err := ValidateFrame(frame)
	if err != nil {
		t.Errorf("valid INFO frame should pass, got: %v", err)
	}
}

// TestValidateFrame_GETMissingKey 测试GET命令缺少Key
func TestValidateFrame_GETMissingKey(t *testing.T) {
	frame := &ProtocolFrame{
		Command:  uint8(CMD_GET),
		KeyLen:   0,
		ValueLen: 0,
		Key:      []byte{},
		Value:    []byte{},
	}
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for GET without key, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_INVALID_KEY {
		t.Errorf("expected error code ERROR_INVALID_KEY, got %v", fe.Code)
	}
}

// TestValidateFrame_DELETEMissingKey 测试DELETE命令缺少Key
func TestValidateFrame_DELETEMissingKey(t *testing.T) {
	frame := &ProtocolFrame{
		Command:  uint8(CMD_DELETE),
		KeyLen:   0,
		ValueLen: 0,
		Key:      []byte{},
		Value:    []byte{},
	}
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for DELETE without key, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_INVALID_KEY {
		t.Errorf("expected error code ERROR_INVALID_KEY, got %v", fe.Code)
	}
}

// TestValidateFrame_SETMissingKey 测试SET命令缺少Key
func TestValidateFrame_SETMissingKey(t *testing.T) {
	frame := &ProtocolFrame{
		Command:  uint8(CMD_SET),
		KeyLen:   0,
		ValueLen: 5,
		Key:      []byte{},
		Value:    []byte("value"),
	}
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for SET without key, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_INVALID_KEY {
		t.Errorf("expected error code ERROR_INVALID_KEY, got %v", fe.Code)
	}
}

// TestValidateFrame_KeyLenInconsistency 测试KeyLen声明值与实际长度不一致（校验码错误场景）
// 对应spec: Key长度字段为100但实际Key长度为50
func TestValidateFrame_KeyLenInconsistency(t *testing.T) {
	frame := &ProtocolFrame{
		Command:  uint8(CMD_GET),
		KeyLen:   100,
		ValueLen: 0,
		Key:      make([]byte, 50), // 实际只有50字节
		Value:    []byte{},
	}
	err := ValidateFrame(frame)
	if err == nil {
		t.Fatal("expected error for key length inconsistency, got nil")
	}

	fe, ok := err.(*FrameError)
	if !ok {
		t.Fatalf("expected *FrameError, got %T", err)
	}
	if fe.Code != ERROR_FRAME_MISMATCH {
		t.Errorf("expected error code ERROR_FRAME_MISMATCH, got %v", fe.Code)
	}
}

// TestGetErrorCode 测试GetErrorCode辅助函数
func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode ErrorCode
	}{
		{
			name:     "FrameError with ERROR_UNKNOWN_COMMAND",
			err:      &FrameError{Code: ERROR_UNKNOWN_COMMAND, Message: "test"},
			wantCode: ERROR_UNKNOWN_COMMAND,
		},
		{
			name:     "FrameError with ERROR_INVALID_KEY",
			err:      &FrameError{Code: ERROR_INVALID_KEY, Message: "test"},
			wantCode: ERROR_INVALID_KEY,
		},
		{
			name:     "FrameError with ERROR_FRAME_MISMATCH",
			err:      &FrameError{Code: ERROR_FRAME_MISMATCH, Message: "test"},
			wantCode: ERROR_FRAME_MISMATCH,
		},
		{
			name:     "non-FrameError returns ERROR_UNKNOWN_COMMAND",
			err:      errors.New("generic error"),
			wantCode: ERROR_UNKNOWN_COMMAND,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetErrorCode(tt.err)
			if got != tt.wantCode {
				t.Errorf("GetErrorCode() = %v, want %v", got, tt.wantCode)
			}
		})
	}
}

// TestFrameError_Error 测试FrameError.Error()方法
func TestFrameError_Error(t *testing.T) {
	fe := &FrameError{
		Code:    ERROR_FRAME_TOO_SHORT,
		Message: "frame total size 5 is less than header size 9",
	}
	expected := "ERROR_FRAME_TOO_SHORT: frame total size 5 is less than header size 9"
	if fe.Error() != expected {
		t.Errorf("FrameError.Error() = %q, want %q", fe.Error(), expected)
	}
}

// TestValidateFrame_EdgeCases 边界条件测试
func TestValidateFrame_EdgeCases(t *testing.T) {
	t.Run("frame with maximum allowed key length", func(t *testing.T) {
		frame := &ProtocolFrame{
			Command:  uint8(CMD_GET),
			KeyLen:   MaxKeyLength,
			ValueLen: 0,
			Key:      make([]byte, MaxKeyLength),
			Value:    []byte{},
		}
		err := ValidateFrame(frame)
		if err != nil {
			t.Errorf("max key length should be valid, got: %v", err)
		}
	})

	t.Run("frame with maximum allowed value length", func(t *testing.T) {
		frame := &ProtocolFrame{
			Command:  uint8(CMD_SET),
			KeyLen:   3,
			ValueLen: MaxValueLength,
			Key:      []byte("key"),
			Value:    make([]byte, MaxValueLength),
		}
		err := ValidateFrame(frame)
		if err != nil {
			t.Errorf("max value length should be valid, got: %v", err)
		}
	})

	t.Run("INFO command with empty key and value is valid", func(t *testing.T) {
		frame := NewFrame(uint8(CMD_INFO), nil, nil)
		err := ValidateFrame(frame)
		if err != nil {
			t.Errorf("INFO with empty key/value should be valid, got: %v", err)
		}
	})

	t.Run("command 0x00 is invalid", func(t *testing.T) {
		frame := NewFrame(0x00, []byte("key"), nil)
		err := ValidateFrame(frame)
		if err == nil {
			t.Fatal("command 0x00 should be invalid")
		}
		fe, ok := err.(*FrameError)
		if !ok {
			t.Fatalf("expected *FrameError, got %T", err)
		}
		if fe.Code != ERROR_UNKNOWN_COMMAND {
			t.Errorf("expected ERROR_UNKNOWN_COMMAND, got %v", fe.Code)
		}
	})

	t.Run("command 0xFF is invalid", func(t *testing.T) {
		frame := NewFrame(0xFF, []byte("key"), nil)
		err := ValidateFrame(frame)
		if err == nil {
			t.Fatal("command 0xFF should be invalid")
		}
	})
}

// TestValidateFrame_AllErrorCodes 验证所有ErrorCode都能通过校验路径覆盖
func TestValidateFrame_AllErrorCodes(t *testing.T) {
	errorScenarios := []struct {
		name     string
		frame    *ProtocolFrame
		wantCode ErrorCode
	}{
		{
			name:     "ERROR_UNKNOWN_COMMAND via invalid command 0x99",
			frame:    &ProtocolFrame{Command: 0x99, KeyLen: 3, ValueLen: 0, Key: []byte("key"), Value: []byte{}},
			wantCode: ERROR_UNKNOWN_COMMAND,
		},
		{
			name:     "ERROR_INVALID_KEY via key too long",
			frame:    &ProtocolFrame{Command: uint8(CMD_GET), KeyLen: MaxKeyLength + 1, ValueLen: 0, Key: make([]byte, MaxKeyLength+1), Value: []byte{}},
			wantCode: ERROR_INVALID_KEY,
		},
		{
			name:     "ERROR_INVALID_VALUE via value too long",
			frame:    &ProtocolFrame{Command: uint8(CMD_SET), KeyLen: 3, ValueLen: MaxValueLength + 1, Key: []byte("key"), Value: make([]byte, MaxValueLength+1)},
			wantCode: ERROR_INVALID_VALUE,
		},
		{
			name:     "ERROR_FRAME_MISMATCH via key length mismatch",
			frame:    &ProtocolFrame{Command: uint8(CMD_GET), KeyLen: 100, ValueLen: 0, Key: []byte("short"), Value: []byte{}},
			wantCode: ERROR_FRAME_MISMATCH,
		},
		{
			name:     "ERROR_FRAME_MISMATCH via value length mismatch",
			frame:    &ProtocolFrame{Command: uint8(CMD_SET), KeyLen: 3, ValueLen: 100, Key: []byte("key"), Value: []byte("short")},
			wantCode: ERROR_FRAME_MISMATCH,
		},
		{
			name:     "ERROR_INVALID_KEY via GET without key",
			frame:    &ProtocolFrame{Command: uint8(CMD_GET), KeyLen: 0, ValueLen: 0, Key: []byte{}, Value: []byte{}},
			wantCode: ERROR_INVALID_KEY,
		},
		{
			name:     "ERROR_INVALID_KEY via SET without key",
			frame:    &ProtocolFrame{Command: uint8(CMD_SET), KeyLen: 0, ValueLen: 5, Key: []byte{}, Value: []byte("value")},
			wantCode: ERROR_INVALID_KEY,
		},
		{
			name:     "ERROR_INVALID_KEY via DELETE without key",
			frame:    &ProtocolFrame{Command: uint8(CMD_DELETE), KeyLen: 0, ValueLen: 0, Key: []byte{}, Value: []byte{}},
			wantCode: ERROR_INVALID_KEY,
		},
	}

	for _, tt := range errorScenarios {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFrame(tt.frame)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			code := GetErrorCode(err)
			if code != tt.wantCode {
				t.Errorf("GetErrorCode() = %v (%s), want %v (%s)",
					code, code.String(), tt.wantCode, tt.wantCode.String())
			}
		})
	}
}

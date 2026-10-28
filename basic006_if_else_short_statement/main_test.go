package main

import (
	"errors"
	"strings"
	"testing"
)

// TestVariableScope demonstrates that variables declared in if statements
// are scoped to the if block. We verify the behavior of our demo functions.
func TestVariableScope(t *testing.T) {
	// This test verifies that errMock and okMock behave as expected.
	// The actual scoping behavior is a language feature, but we can
	// verify the error handling logic.

	t.Run("errMock returns error", func(t *testing.T) {
		_, err := errMock()
		if err == nil {
			t.Fatal("expected error from errMock, got nil")
		}
		if !strings.Contains(err.Error(), "무조건 실패") {
			t.Fatalf("expected '무조건 실패', got %v", err.Error())
		}
	})

	t.Run("okMock returns success", func(t *testing.T) {
		v, err := okMock()
		if err != nil {
			t.Fatalf("unexpected error from okMock: %v", err)
		}
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	})
}

// TestEarlyReturnStyle verifies the early return pattern logic.
func TestEarlyReturnStyle(t *testing.T) {
	tests := []struct {
		name     string
		mockFunc func() (int, error)
		wantErr  bool
		wantValue int
	}{
		{
			name:     "error case",
			mockFunc: errMock,
			wantErr:  true,
		},
		{
			name:     "success case",
			mockFunc: okMock,
			wantErr:  false,
			wantValue: 42,
		},
		{
			name:     "negative value case",
			mockFunc: negativeMock,
			wantErr:  false,
			wantValue: -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := tt.mockFunc()
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error: got %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && v != tt.wantValue {
				t.Errorf("unexpected value: got %d, want %d", v, tt.wantValue)
			}
		})
	}
}

// TestHandleWithEarlyReturn verifies the early return logic in handleWithEarlyReturn.
// We can't directly test the output, but we can verify the preconditions.
func TestHandleWithEarlyReturn(t *testing.T) {
	// Verify that okMock returns the expected value for the success path
	v, err := okMock()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
	if v <= 0 {
		t.Fatal("expected positive value, got non-positive")
	}
}

// TestFileCheck verifies the file check logic.
func TestFileCheck(t *testing.T) {
	tests := []struct {
		name string
		path string
		exist bool
	}{
		{
			name: "non-existent file",
			path: "/tmp/non_existent_file_go_tour.txt",
			exist: false,
		},
		{
			name: "existing file (hostname)",
			path: "/etc/hostname",
			exist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We verify the os.Stat behavior directly
			info, err := os.Stat(tt.path)
			if tt.exist {
				if err != nil {
					t.Fatalf("expected file to exist, got error: %v", err)
				}
				if info == nil {
					t.Fatal("expected non-nil FileInfo")
				}
			} else {
				if err == nil {
					t.Fatal("expected error for non-existent file, got nil")
				}
				if !os.IsNotExist(err) {
					t.Fatalf("expected IsNotExist error, got: %v", err)
				}
			}
		})
	}
}

// TestMockG verifies the mockG function.
func TestMockG(t *testing.T) {
	tests := []struct {
		name string
		v    int
		wantErr bool
	}{
		{
			name: "valid value",
			v: 42,
			wantErr: false,
		},
		{
			name: "value too large",
			v: 101,
			wantErr: true,
		},
		{
			name: "zero value",
			v: 0,
			wantErr: false, // mockG only checks > 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mockG(tt.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error: got %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "너무 큼") {
				t.Fatalf("expected '너무 큼' error, got: %v", err.Error())
			}
		})
	}
}
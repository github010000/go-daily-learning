package main

import (
	"runtime"
	"testing"
)

// TestRecommendedGOMAXPROCS: limit 값에 따라 권장 GOMAXPROCS가 올바르게 계산되는지 검증한다.
// 소수점 이하는 올림 처리되어야 하며, limit=0(무제한)이면 호스트 CPU 수를 반환해야 한다.
func TestRecommendedGOMAXPROCS(t *testing.T) {
	tests := []struct {
		name  string
		limit float64
		want  int
	}{
		{"unlimited_zero", 0, runtime.NumCPU()},
		{"one_cpu", 1.0, 1},
		{"fraction_round_up", 1.2, 2},
		{"exact_two", 2.0, 2},
		{"five_point_seven", 5.7, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recommendedGOMAXPROCS(tt.limit)
			if got != tt.want {
				t.Errorf("recommendedGOMAXPROCS(%v) = %d, want %d", tt.limit, got, tt.want)
			}
		})
	}
}

// TestParseCPUmaxV2: cgroup v2 cpu.max 파서가 무제한, 정상 quota, 잘못된 형식을 올바르게 처리하는지 검증한다.
// 이 파서는 detectContainerCPULimit의 핵심 로직이므로 정확성이 중요하다.
func TestParseCPUmaxV2(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantLimit     float64
		wantUnlimited bool
		wantErr       bool
	}{
		{"unlimited", "max 100000", 0, true, false},
		{"two_cpus", "200000 100000", 2.0, false, false},
		{"half_cpu", "50000 100000", 0.5, false, false},
		{"invalid_format", "200000", 0, false, true},
		{"negative_period", "200000 -100", 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, unlimited, err := parseCPUmaxV2(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseCPUmaxV2(%q) expected error, got nil", tt.content)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCPUmaxV2(%q) unexpected error: %v", tt.content, err)
			}
			if limit != tt.wantLimit {
				t.Errorf("limit = %v, want %v", limit, tt.wantLimit)
			}
			if unlimited != tt.wantUnlimited {
				t.Errorf("unlimited = %v, want %v", unlimited, tt.wantUnlimited)
			}
		})
	}
}
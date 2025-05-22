package gorp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseQualityGate(t *testing.T) {
	type args struct {
		metadata map[string]any
	}
	tests := []struct {
		name  string
		args  args
		want  *QualityGate
		want1 bool
	}{
		{
			name:  "nil metadata",
			args:  args{metadata: nil},
			want:  nil,
			want1: false,
		},
		{
			name:  "empty metadata",
			args:  args{metadata: map[string]any{}},
			want:  nil,
			want1: false,
		},
		{
			name: "qualityGate not a map",
			args: args{metadata: map[string]any{
				"qualityGate": "not a map",
			}},
			want:  nil,
			want1: false,
		},
		{
			name: "id as int64",
			args: args{metadata: map[string]any{
				"qualityGate": map[string]any{
					"id":     int64(123),
					"status": "PASSED",
				},
			}},
			want:  &QualityGate{ID: 123, Status: "PASSED"},
			want1: true,
		},
		{
			name: "id as float64",
			args: args{metadata: map[string]any{
				"qualityGate": map[string]any{
					"id":     float64(456),
					"status": "FAILED",
				},
			}},
			want:  &QualityGate{ID: 456, Status: "FAILED"},
			want1: true,
		},
		{
			name: "id as string (valid)",
			args: args{metadata: map[string]any{
				"qualityGate": map[string]any{
					"id":     "789",
					"status": "WARNING",
				},
			}},
			want:  &QualityGate{ID: 789, Status: "WARNING"},
			want1: true,
		},
		{
			name: "id as string (invalid)",
			args: args{metadata: map[string]any{
				"qualityGate": map[string]any{
					"id":     "not-a-number",
					"status": "UNKNOWN",
				},
			}},
			want:  nil,
			want1: false,
		},
		{
			name: "id missing",
			args: args{metadata: map[string]any{
				"qualityGate": map[string]any{
					"status": "PASSED",
				},
			}},
			want:  nil,
			want1: false,
		},
		{
			name: "status missing",
			args: args{metadata: map[string]any{
				"qualityGate": map[string]any{
					"id": int64(123),
				},
			}},
			want:  &QualityGate{ID: 123, Status: ""},
			want1: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ParseQualityGate(tt.args.metadata)
			assert.Equalf(t, tt.want, got, "ParseQualityGate(%v)", tt.args.metadata)
			assert.Equalf(t, tt.want1, got1, "ParseQualityGate(%v)", tt.args.metadata)
		})
	}
}

//go:build integration
// +build integration

package access

import (
	"io"
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/stretchr/testify/assert"
)

func TestPolicyEvaluation(t *testing.T) {
	t.Parallel()

	l := logger.NewUPPLogger("test", "panic")
	l.Out = io.Discard

	oa := GetOPAAgentForTesting(l)

	tests := []struct {
		name        string
		input       map[string]interface{}
		expected    bool
		expectedErr error
	}{
		{
			name:        "allow default",
			input:       map[string]interface{}{},
			expected:    true,
			expectedErr: nil,
		},
		{
			name:        "deny for central banking editorial desk",
			input:       map[string]interface{}{"EditorialDesk": "/FT/Professional/Central Banking"},
			expected:    false,
			expectedErr: nil,
		},
		{
			name:        "deny for sustainable views publication",
			input:       map[string]interface{}{"Publication": "8e6c705e-1132-42a2-8db0-c295e29e8658"},
			expected:    false,
			expectedErr: nil,
		},
		{
			name:        "deny for fta publication",
			input:       map[string]interface{}{"Publication": "19d50190-8656-4e91-8d34-82e646ada9c9"},
			expected:    false,
			expectedErr: nil,
		},
		{
			name:        "allow for other editorial desk",
			input:       map[string]interface{}{"EditorialDesk": "/FT/Professional/Other"},
			expected:    true,
			expectedErr: nil,
		},
		{
			name:        "allow for other publication",
			input:       map[string]interface{}{"Publication": "some-other-publication-id"},
			expected:    true,
			expectedErr: nil,
		},
		{
			name: "deny for both criteria met",
			input: map[string]interface{}{
				"EditorialDesk": "/FT/Professional/Central Banking",
				"Publication":   "8e6c705e-1132-42a2-8db0-c295e29e8658",
			},
			expected:    false,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := oa.EvaluateContentPolicy(tt.input)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				if assert.NotNil(t, result) {
					assert.Equal(t, tt.expected, result.Allow)
				}
			}
		})
	}
}

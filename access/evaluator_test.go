package access

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/rego"
	"github.com/stretchr/testify/assert"
)

func TestEvaluator_EvaluateNotificationAccessLevel(t *testing.T) {
	defaultEvalQuery, err := rego.New(
		rego.Query("data.specialContent.allow"),
		rego.Load([]string{"../opa_modules/special_content.rego"}, nil),
	).PrepareForEval(context.TODO())
	assert.NoError(t, err)

	tests := []struct {
		name         string
		evalQuery    rego.PreparedEvalQuery
		notification map[string]interface{}
		want         bool
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name:      "Basic special content notification",
			evalQuery: defaultEvalQuery,
			notification: map[string]interface{}{
				"EditorialDesk": "/FT/Professional/Central Banking",
			},
			want:    false,
			wantErr: assert.NoError,
		},
		{
			name:      "Basic non-special banking notification",
			evalQuery: defaultEvalQuery,
			notification: map[string]interface{}{
				"EditorialDesk": "/FT/Newsletters",
			},
			want:    true,
			wantErr: assert.NoError,
		},
		{
			name:         "Missing editorial desk field",
			evalQuery:    defaultEvalQuery,
			notification: map[string]interface{}{},
			want:         true,
			wantErr:      assert.NoError,
		},
		{
			name:      "SustainableViews notification",
			evalQuery: defaultEvalQuery,
			notification: map[string]interface{}{
				"Publication": "8e6c705e-1132-42a2-8db0-c295e29e8658",
			},
			want:    false,
			wantErr: assert.NoError,
		},
		{
			name:      "FT Pink notification",
			evalQuery: defaultEvalQuery,
			notification: map[string]interface{}{
				"Publication": "88fdde6c-2aa4-4f78-af02-9f680097cfd6",
			},
			want:    true,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Evaluator{
				evalQuery: tt.evalQuery,
			}
			got, err := e.EvaluateNotificationAccessLevel(tt.notification)
			if !tt.wantErr(t, err, fmt.Sprintf("EvaluateNotificationAccessLevel(%v)", tt.notification)) {
				return
			}
			assert.Equalf(t, tt.want, got, "EvaluateNotificationAccessLevel(%v)", tt.notification)
		})
	}
}

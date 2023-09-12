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
		rego.Query("data.centralBanking.allow"),
		rego.Load([]string{"../opa_modules/central_banking.rego"}, nil),
	).PrepareForEval(context.TODO())
	assert.NoError(t, err)

	type notification struct {
		EditorialDesk string
	}

	tests := []struct {
		name         string
		evalQuery    rego.PreparedEvalQuery
		notification interface{}
		want         bool
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name:      "Basic central banking notification",
			evalQuery: defaultEvalQuery,
			notification: notification{
				EditorialDesk: "/FT/Professional/Central Banking",
			},
			want:    false,
			wantErr: assert.NoError,
		},
		{
			name:      "Basic non-central banking notification",
			evalQuery: defaultEvalQuery,
			notification: notification{
				EditorialDesk: "/FT/Newsletters",
			},
			want:    true,
			wantErr: assert.NoError,
		},
		{
			name:         "Missing editorial desk field",
			evalQuery:    defaultEvalQuery,
			notification: notification{},
			want:         true,
			wantErr:      assert.NoError,
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

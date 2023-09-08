package access

import (
	"context"

	"github.com/open-policy-agent/opa/rego"
)

type Evaluator struct {
	evalQuery rego.PreparedEvalQuery
}

func CreateEvaluator(query string, moduleLocation []string) (*Evaluator, error) {
	evalQuery, err := rego.New(
		rego.Query(query),
		rego.Load(moduleLocation, nil),
	).PrepareForEval(context.TODO())

	if err != nil {
		return nil, err
	}

	return &Evaluator{evalQuery: evalQuery}, nil
}

func (e *Evaluator) EvaluateNotificationAccessLevel(notification interface{}) (bool, error) {
	eval, err := e.evalQuery.Eval(context.TODO(), rego.EvalInput(notification))
	if err != nil {
		return false, err
	}

	return eval.Allowed(), nil
}

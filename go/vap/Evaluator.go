// Copyright 2023 Undistro Authors
//matchConditionsCelVar

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vap

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"sigs.k8s.io/yaml"
)

var celEnvOptions = []cel.EnvOption{
	cel.EagerlyValidateDeclarations(true),
	cel.DefaultUTCTimeZone(true),
	ext.Strings(ext.StringsVersion(2)),
	cel.CrossTypeNumericComparisons(true),
	cel.OptionalTypes(),

	// including the following dependencies breaks the WASM install. To many deps?
	// library.URLs(),
	// library.Regex(),
	// library.Lists(),
	// library.Quantity(),
}

type EvalResult struct {
	Name    string  `json:"name,omitempty"`
	Result  ref.Val `json:"result,omitempty"`
	Error   string  `json:"error,omitempty"`
	Message any     `json:"message,omitempty"`
}

type EvaluationResults struct {
	Error            string        `json:"error,omitempty"`
	Variables        []*EvalResult `json:"variables,omitempty"`
	MatchConditions  []*EvalResult `json:"matchConditions,omitempty"`
	Validations      []*EvalResult `json:"validations,omitempty"`
	AuditAnnotations []*EvalResult `json:"auditAnnotations,omitempty"`

	TypeChecking *TypeChecking `json:"typeChecking,omitempty"`
}

type ObjectData map[string]any

// AdmissionPolicyEvaluator holds policy, data and results for a single validation run
type AdmissionPolicyEvaluator struct {
	policy       ValidatingAdmissionPolicy
	object       ObjectData
	oldObject    ObjectData
	paramsObject ObjectData

	celEnvironment *cel.Env

	results EvaluationResults
}

// 'object' - The object from the incoming request. The value is null for DELETE requests.
// 'oldObject' - The existing object. The value is null for CREATE requests.
// 'request' - Attributes of the admission request.
// 'params' - Parameter resource referred to by the policy binding being evaluated. The value is null if ParamKind is not specified.
// namespaceObject - The namespace, as a Kubernetes resource, that the incoming object belongs to. The value is null if the incoming object is cluster-scoped.
// authorizer - A CEL Authorizer. May be used to perform authorization checks for the principal (authenticated user) of the request. See AuthzSelectors and Authz in the Kubernetes CEL library documentation for more details.
// authorizer.requestResource - A shortcut for an authorization check configured with the request resource (group, resource, (subresource), namespace, name).
func NewAdmissionPolicyEvaluator(policy, object, oldObject, request, params, namespace []byte) (*AdmissionPolicyEvaluator, error) {

	evaluator := AdmissionPolicyEvaluator{
		results: EvaluationResults{TypeChecking: &TypeChecking{}},
	}

	if err := yaml.Unmarshal(policy, &evaluator.policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy YAML: %w", err)
	}

	if err := yaml.Unmarshal(object, &evaluator.object); err != nil {
		return nil, fmt.Errorf("failed to parse object YAML: %w", err)
	}

	if err := yaml.Unmarshal(oldObject, &evaluator.oldObject); err != nil {
		return nil, fmt.Errorf("failed to parse oldObject YAML: %w", err)
	}

	if err := yaml.Unmarshal(params, &evaluator.paramsObject); err != nil {
		return nil, fmt.Errorf("failed to parse params YAML: %w", err)
	}

	// Declare input data as variables
	variables := []cel.EnvOption{
		cel.Variable("object", cel.DynType),
		cel.Variable("params", cel.DynType),
	}
	// Declare additional variables
	for _, variable := range evaluator.policy.Spec.Variables {
		variables = append(variables, cel.Variable(variable.Name, cel.DynType))
	}

	// Init CEL environment
	if celEnvironment, err := cel.NewEnv(append(celEnvOptions, variables...)...); err == nil {
		evaluator.celEnvironment = celEnvironment
	} else {
		return nil, fmt.Errorf("failed to create CEL env: %w", err)
	}

	return &evaluator, nil
}

func (evaluator *AdmissionPolicyEvaluator) Evaluate() (string, error) {

	// Input Variables
	inputData := map[string]any{
		"object": evaluator.object,
		"params": evaluator.paramsObject,
	}

	// Policy variables
	for _, variable := range evaluator.policy.Spec.Variables {
		value := evaluator.evalExpression(inputData, variable.Expression)
		inputData["variables."+variable.Name] = value
		evaluator.results.Variables = append(evaluator.results.Variables, &EvalResult{
			Name:   variable.Name,
			Result: value,
		})
	}

	// MatchConditions
	for _, matchCondition := range evaluator.policy.Spec.MatchConditions {
		value := evaluator.evalExpression(inputData, matchCondition.Expression)
		if types.IsUnknownOrError(value) {
			if err, ok := value.Value().(error); ok {
				warning := ExpressionWarning{
					FieldRef: fmt.Sprintf("spec.matchConditions[%s].expression", matchCondition.Name),
					Warning:  err.Error(),
				}
				evaluator.results.TypeChecking.ExpressionWarnings = append(evaluator.results.TypeChecking.ExpressionWarnings, warning)
			}
		}
		evaluator.results.MatchConditions = append(evaluator.results.MatchConditions, &EvalResult{
			Name:   matchCondition.Name,
			Result: value,
		})
	}

	// Validations
	if isValidResult(evaluator.results.MatchConditions) {
		for idx, validation := range evaluator.policy.Spec.Validations {
			value := evaluator.evalExpression(inputData, validation.Expression)
			evalResult := &EvalResult{
				Name:   fmt.Sprintf("spec.validations[%d].expression", idx),
				Result: value,
			}

			if types.IsUnknownOrError(value) {
				if err, ok := evalResult.Result.Value().(error); ok {
					warning := ExpressionWarning{
						FieldRef: fmt.Sprintf("spec.validations[%d].expression", idx),
						Warning:  err.Error(),
					}
					evaluator.results.TypeChecking.ExpressionWarnings = append(evaluator.results.TypeChecking.ExpressionWarnings, warning)
				}
			} else if value.Value() != true {
				evalResult.Message = validation.Message

				if evalResult.Message == "" && validation.MessageExpression != "" {
					message := evaluator.evalExpression(inputData, validation.MessageExpression)
					if !types.IsUnknownOrError(message) {
						evalResult.Message = message
					}
				}
			}

			evaluator.results.Validations = append(evaluator.results.Validations, evalResult)
		}

		// AuditAnnotations
		for _, auditAnnotation := range evaluator.policy.Spec.AuditAnnotations {
			value := evaluator.evalExpression(inputData, auditAnnotation.ValueExpression)
			evaluator.results.AuditAnnotations = append(evaluator.results.AuditAnnotations, &EvalResult{
				Name:   auditAnnotation.Key,
				Result: value,
			})
		}

	}

	data, err := json.Marshal(evaluator.results)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (evaluator *AdmissionPolicyEvaluator) evalExpression(inputData map[string]any, expression string) ref.Val {
	ast, issues := evaluator.celEnvironment.Compile(expression)
	if issues.Err() != nil {
		log.Printf("Compile: %v", issues.String())
		return types.WrapErr(issues.Err())
	}
	prog, err := evaluator.celEnvironment.Program(ast)
	if err != nil {
		log.Printf("Program: %v", err)
		return types.WrapErr(err)
	}
	result, _, err := prog.Eval(inputData)
	if err != nil {
		log.Printf("Eval %v", err)
		return types.WrapErr(err)
	}
	return result
}

func isValidResult(results []*EvalResult) bool {
	for _, result := range results {
		if result.Result.Value() != true {
			return false
		}
	}
	return true
}

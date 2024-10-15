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

var celProgramOptions = []cel.ProgramOption{
	cel.EvalOptions(cel.OptOptimize, cel.OptTrackCost),
}

var celEnvOptions = []cel.EnvOption{
	cel.EagerlyValidateDeclarations(true),
	cel.DefaultUTCTimeZone(true),
	ext.Strings(ext.StringsVersion(2)),
	cel.CrossTypeNumericComparisons(true),
	cel.OptionalTypes(),
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
	Variables        []*EvalResult `json:"variables,omitempty"`
	MatchConditions  []*EvalResult `json:"matchConditions,omitempty"`
	Validations      []*EvalResult `json:"validations,omitempty"`
	AuditAnnotations []*EvalResult `json:"auditAnnotations,omitempty"`
}

type AdmissionPolicyEvaluator struct {
	policy         *ValidatingAdmissionPolicy
	objectValue    map[string]any
	oldObjectValue map[string]any

	celEnvironment *cel.Env

	results EvaluationResults
}

func NewAdmissionPolicyEvaluator(policyInput, oldObjectInput, objectValueInput, namespaceInput, requestInput, authorizerInput []byte) (*AdmissionPolicyEvaluator, error) {

	evaluator := AdmissionPolicyEvaluator{
		policy: &ValidatingAdmissionPolicy{},
	}

	if err := yaml.Unmarshal(policyInput, evaluator.policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy: %w", err)
	}

	if err := yaml.Unmarshal(oldObjectInput, &evaluator.oldObjectValue); err != nil {
		return nil, fmt.Errorf("failed to parse old resource: %w", err)
	}

	if err := yaml.Unmarshal(objectValueInput, &evaluator.objectValue); err != nil {
		return nil, fmt.Errorf("failed to parse resource: %w", err)
	}

	// Declare input data as variables
	variables := []cel.EnvOption{
		cel.Variable("object", cel.DynType),
		cel.Variable("request", cel.DynType),
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

func (evaluator *AdmissionPolicyEvaluator) evalExpression(inputData map[string]any, variableName string, expression string) ref.Val {
	ast, issues := evaluator.celEnvironment.Parse(expression)
	if issues.Err() != nil {
		log.Printf("Issues %v", issues.Err())
		return types.WrapErr(issues.Err())
	}

	// ??  Checked: ERROR: <input>:1:1: undeclared reference to 'variables' (in container '') variables.isDeployment
	// if _, issues := evaluator.celEnvironment.Check(ast); issues.Err() != nil {
	// 	log.Printf("Checked: %v", issues.Err())
	// 	return types.WrapErr(issues.Err())
	// }
	prog, err := evaluator.celEnvironment.Program(ast, celProgramOptions...)
	if err != nil {
		log.Printf("Program: %v", err)
		return types.WrapErr(err)
	}
	val, _, err := prog.Eval(inputData)
	if err != nil {
		log.Printf("Eval %v", err)
		return types.WrapErr(err)
	}
	return val
}

func (evaluator *AdmissionPolicyEvaluator) Evaluate() (string, error) {

	// Input Variables
	inputData := map[string]any{
		"object":  evaluator.objectValue,
		"request": evaluator.objectValue,
	}

	// Admission Policy variables
	for _, variable := range evaluator.policy.Spec.Variables {
		value := evaluator.evalExpression(inputData, variable.Name, variable.Expression)
		inputData["variables."+variable.Name] = value
		evaluator.results.Variables = append(evaluator.results.Variables, &EvalResult{
			Name:   variable.Name,
			Result: value,
		})
	}

	// Evaluate MatchConditions
	for _, matchCondition := range evaluator.policy.Spec.MatchConditions {
		value := evaluator.evalExpression(inputData, matchCondition.Name, matchCondition.Expression)
		evaluator.results.MatchConditions = append(evaluator.results.MatchConditions, &EvalResult{
			Name:   matchCondition.Name,
			Result: value,
		})
	}

	if isValidResult(evaluator.results.MatchConditions) {

		// Validations
		for _, validation := range evaluator.policy.Spec.Validations {
			value := evaluator.evalExpression(inputData, "", validation.Expression)

			evalResult := &EvalResult{
				Name:   "",
				Result: value,
			}
			if value.Value() != true {
				if validation.Message != "" {
					evalResult.Message = validation.Message
				} else if validation.MessageExpression != "" {
					evalResult.Message = evaluator.evalExpression(inputData, "", validation.Expression)
				}
			}
			evaluator.results.Validations = append(evaluator.results.Validations, evalResult)
		}

		// AuditAnnotations
		if isValidResult(evaluator.results.Validations) {
			for _, auditAnnotation := range evaluator.policy.Spec.AuditAnnotations {
				evalResult := evaluator.evalExpression(inputData, auditAnnotation.Key, auditAnnotation.ValueExpression)
				evaluator.results.AuditAnnotations = append(evaluator.results.AuditAnnotations, &EvalResult{
					Name:   auditAnnotation.Key,
					Result: evalResult,
				})
			}
		}
	}

	return evaluator.generateResponse(inputData)
}

func isValidResult(results []*EvalResult) bool {
	for _, result := range results {
		if result.Result.Value() != true {
			return false
		}
	}
	return true
}

func unWrapErrors(evalResults []*EvalResult) {
	for _, evalResult := range evalResults {
		if types.IsUnknownOrError(evalResult.Result) {
			if err, ok := evalResult.Result.Value().(error); ok {
				evalResult.Error = err.Error()
			}
		}
	}
}

func (evaluator *AdmissionPolicyEvaluator) generateResponse(inputData map[string]any) (string, error) {

	unWrapErrors(evaluator.results.Variables)
	unWrapErrors(evaluator.results.MatchConditions)
	unWrapErrors(evaluator.results.Validations)
	unWrapErrors(evaluator.results.AuditAnnotations)

	data, err := json.Marshal(evaluator.results)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

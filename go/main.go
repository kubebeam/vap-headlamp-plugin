//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"vap-headlamp-plugin/vap"
)

func main() {
	// Create a channel to keep the Go program alive
	done := make(chan struct{}, 0)

	// Expose the Go functions to JavaScript
	js.Global().Set("AdmissionEval", js.FuncOf(admissionEval))

	// Block the program from exiting
	<-done
}

func errorToJSON(err error) string {
	results := vap.EvaluationResults{
		Error: err.Error(),
	}
	data, _ := json.Marshal(results)
	return string(data)
}

func admissionEval(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return errorToJSON(fmt.Errorf("Invalid no of arguments passed"))
	}
	policy := []byte(args[0].String())
	resource := []byte(args[1].String())
	//request := args[1]
	params := []byte(args[2].String())
	//namespaceObject :=
	evaluator, err := vap.NewAdmissionPolicyEvaluator(policy, resource, nil, nil, params, nil)
	if err != nil {
		fmt.Printf("unable to parse inputs %s\n", err)
		return errorToJSON(err)
	}
	result, err := evaluator.Evaluate()
	if err != nil {
		fmt.Printf("unable to validate admission policy %s\n", err)
		return errorToJSON(err)
	}
	return result
}

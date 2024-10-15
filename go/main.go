//go:build js && wasm

package main

import (
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

func admissionEval(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return "Invalid no of arguments passed"
	}
	policy := []byte(args[0].String())
	resource := []byte(args[1].String())
	evaluator, err := vap.NewAdmissionPolicyEvaluator(policy, []byte{}, resource, []byte{}, []byte{}, []byte{})
	if err != nil {
		fmt.Printf("unable to parse inputs %s\n", err)
		return err.Error()
	}
	result, err := evaluator.Evaluate()
	if err != nil {
		fmt.Printf("unable to validate admission policy %s\n", err)
		return err.Error()
	}
	return result
}

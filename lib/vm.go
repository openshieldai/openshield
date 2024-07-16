package lib

import (
	"encoding/json"

	"github.com/dop251/goja"
)

// JavaScript code that acts like a Lambda function
const jsLambdaFunc = `
function handler(event) {
    var input = JSON.parse(event);
    return '{"model":"gpt-4","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"What is the meaning of life?"}]}';
}
`

func Javascript() ([]byte, error) {
	vm := goja.New()
	_, err := vm.RunString(jsLambdaFunc)
	if err != nil {
		panic(err)
	}

	// Simulate an event that you would receive in AWS Lambda
	input := map[string]string{
		"name": "John",
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}

	// Get the JavaScript function and run it
	var outputJSON goja.Value
	if fn, ok := goja.AssertFunction(vm.Get("handler")); ok {
		outputJSON, err = fn(goja.Undefined(), vm.ToValue(string(inputJSON)))
		if err != nil {
			panic(err)
		}
	}

	// Convert the output back to Go struct
	var output map[string]interface{}
	err = json.Unmarshal([]byte(outputJSON.String()), &output)
	if err != nil {
		return nil, err
	}

	outputBytes, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}

	return outputBytes, nil
}

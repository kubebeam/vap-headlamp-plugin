.PHONY: download
download: 
	curl https://raw.githubusercontent.com/golang/go/refs/heads/master/lib/wasm/wasm_exec.js -o src/validating-admission/wasm_exec.js

.PHONY: build
build: 
	GOOS=js GOARCH=wasm go -C go build -ldflags="-s -w" -o ../dist/main.wasm main.go 

.PHONY: local
local: build
	cp dist/main.wasm ~/.config/Headlamp/plugins/vap-plugin/

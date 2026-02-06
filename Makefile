.PHONY: init gen clean

# Install Go tools
init:
	@echo ">>> 🛠️  Installing Generators..."
	cd conscience_go && go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway
	cd conscience_go && go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2
	cd conscience_go && go install google.golang.org/protobuf/cmd/protoc-gen-go
	cd conscience_go && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

# Generate Code (Go + Python)
gen:
	@echo ">>> 🧠 Generating Nervous System..."
	
	# 1. Generate Go (Server + Gateway)
	# Note: We use the installed protoc-gen-go binaries found in $(GOBIN) or $GOPATH/bin
	protoc -I . \
		--go_out ./conscience_go/internal/protocol --go_opt paths=source_relative \
		--go-grpc_out ./conscience_go/internal/protocol --go-grpc_opt paths=source_relative \
		--grpc-gateway_out ./conscience_go/internal/protocol --grpc-gateway_opt paths=source_relative \
		ghost.proto

	# 2. Generate Python (Brain Client)
	# Requires: pip install grpcio-tools
	python -m grpc_tools.protoc -I. --python_out=./brain_python/brain --grpc_python_out=./brain_python/brain ghost.proto

clean:
	rm -f conscience_go/internal/protocol/*.pb.go
	rm -f conscience_go/internal/protocol/*.gw.go
	rm -f brain_python/brain/ghost_pb2*.py

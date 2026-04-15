# ----------------------------
# Variables
# ----------------------------
PROTO_DIR=proto
PROTO_FILE=$(PROTO_DIR)/route.proto

GO_OUT=.
GRPC_OUT=.

BINARY_NAME=shipment-service
AI_BINARY_NAME=ai-service

.PHONY: proto clean-proto build run test clean
.PHONY: ai-build ai-run ai-test

# ----------------------------
# Protobuf Generation
# ----------------------------
proto:
	protoc \
	--go_out=$(GO_OUT) \
	--go-grpc_out=$(GRPC_OUT) \
	$(PROTO_FILE)

# ----------------------------
# Clean Generated Files
# ----------------------------
clean-proto:
	rm -f $(PROTO_DIR)/*.pb.go

# ----------------------------
# Build Binary
# ----------------------------
build:
	go build -o bin/$(BINARY_NAME) main.go

# ----------------------------
# Run Service
# ----------------------------
run:
	go run cmd/main.go

# ----------------------------
# Test
# ----------------------------
test:
	go test ./...

# ----------------------------
# AI Service
# ----------------------------
ai-build:
	python3 -m compileall ai-service/app

ai-run:
	uvicorn app.main:app --app-dir ai-service --host 0.0.0.0 --port 8085

ai-test:
	PYTHONPATH=ai-service python3 -m unittest discover -s ai-service/tests -p 'test_*.py'

# ----------------------------
# Clean Build
# ----------------------------
clean:
	rm -rf bin/

BIN_DIR=bin
BIN_NAME=server
PKG=./cmd/server

.PHONY: build clean run

build:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(BIN_NAME) $(PKG)

clean:
	rm -rf $(BIN_DIR)

run:
	./$(BIN_DIR)/$(BIN_NAME)



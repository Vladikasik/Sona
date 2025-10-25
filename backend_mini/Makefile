APP_NAME := backend_mini
BIN_DIR := bin
DATA_DIR := data

.PHONY: all build run clean

all: build

build:
	mkdir -p $(BIN_DIR) $(DATA_DIR)
	cd cmd/server && go build -o ../../$(BIN_DIR)/$(APP_NAME)

run: build
	./$(BIN_DIR)/$(APP_NAME)

clean:
	rm -rf $(BIN_DIR)



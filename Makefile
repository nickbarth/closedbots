.DEFAULT_GOAL := run

GO ?= go
APP_NAME ?= closedbots
CMD_PATH ?= ./cmd/closedbots
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/$(APP_NAME)
TAGS ?= robotgo
GOCACHE ?= /tmp/go-cache

.PHONY: run build test clean

run: build
	./$(BIN)

build:
	mkdir -p $(BIN_DIR)
	mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) $(GO) build -tags "$(TAGS)" -o $(BIN) $(CMD_PATH)

test:
	mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) $(GO) test ./...

clean:
	rm -rf $(BIN_DIR)
	rm -f $(APP_NAME)

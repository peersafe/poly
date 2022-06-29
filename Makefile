GOFMT=gofmt
GC=go build
VERSION:=$(shell git describe --always)
BUILD_NODE_PAR = -ldflags "-X github.com/polynetwork/poly/common/config.Version=$(VERSION)" #-race

ARCH=$(shell uname -s)
DBUILD=docker build
DRUN=docker run
DOCKER_NS ?= cross-network
DOCKER_TAG=$(ARCH)-$(VERSION)

SRC_FILES = $(shell git ls-files | grep -e .go | grep -v _test.go)
TOOLS=./tools
ABI=$(TOOLS)/abi
NATIVE_ABI_SCRIPT=./cmd/abi/native_abi_script

poly: $(SRC_FILES)
	$(GC)  $(BUILD_NODE_PAR) -o build/bin/poly main.go
 
sigsvr: $(SRC_FILES) abi 
	$(GC)  $(BUILD_NODE_PAR) -o build/bin/sigsvr sigsvr.go
	@if [ ! -d $(TOOLS) ];then mkdir -p $(TOOLS) ;fi
	@mv build/bin/sigsvr build/bin/$(TOOLS)

abi: 
	@if [ ! -d $(ABI) ];then mkdir -p $(ABI) ;fi
	@cp $(NATIVE_ABI_SCRIPT)/*.json $(ABI)

tools: sigsvr abi

all: poly tools

poly-cross: poly-windows poly-linux poly-darwin

poly-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GC) $(BUILD_NODE_PAR) -o build/bin/poly-windows-amd64.exe main.go

poly-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GC) $(BUILD_NODE_PAR) -o build/bin/poly-linux-amd64 main.go

poly-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GC) $(BUILD_NODE_PAR) -o build/bin/poly-darwin-amd64 main.go

tools-cross: tools-windows tools-linux tools-darwin

tools-windows: abi 
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GC) $(BUILD_NODE_PAR) -o build/bin/sigsvr-windows-amd64.exe sigsvr.go
	@if [ ! -d $(TOOLS) ];then mkdir -p $(TOOLS) ;fi
	@mv sigsvr-windows-amd64.exe $(TOOLS)

tools-linux: abi 
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GC) $(BUILD_NODE_PAR) -o build/bin/sigsvr-linux-amd64 sigsvr.go
	@if [ ! -d $(TOOLS) ];then mkdir -p $(TOOLS) ;fi
	@mv sigsvr-linux-amd64 $(TOOLS)

tools-darwin: abi 
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GC) $(BUILD_NODE_PAR) -o build/bin/sigsvr-darwin-amd64 sigsvr.go
	@if [ ! -d $(TOOLS) ];then mkdir -p $(TOOLS) ;fi
	@mv build/bin/sigsvr-darwin-amd64 build/bin/$(TOOLS)

all-cross: poly-cross tools-cross abi

format:
	$(GOFMT) -w main.go

images: Makefile
	@echo "Building poly docker image"
	@$(DBUILD) --no-cache -f docker/Dockerfile -t $(DOCKER_NS):$(DOCKER_TAG) .
	@docker tag $(DOCKER_NS):$(DOCKER_TAG) $(DOCKER_NS):latest

clean:
	rm -rf *.8 *.o *.out *.6 *exe
	rm -rf poly poly-* tools docker/payload docker/build build/bin

accountAdd:
	go run main.go account add --wallet ./build/node1/wallet.dat -d
	go run main.go account add --wallet ./build/node2/wallet.dat -d
	go run main.go account add --wallet ./build/node3/wallet.dat -d
	go run main.go account add --wallet ./build/node4/wallet.dat -d

runNode1:
	go run main.go --data-dir ./build/node1/chain --enable-consensus --loglevel 1 --networkid 4 --rest --wallet ./build/node1/wallet/wallet.dat --rpcport 21336 --rest --restport 21334 --consensus-port 21339 --nodeport 21338

runNode2:
	go run main.go --data-dir ./build/node2/chain --enable-consensus --loglevel 1 --networkid 4 --rest --wallet ./build/node2/wallet/wallet.dat --rpcport 22336 --rest --restport 22334 --consensus-port 22339 --nodeport 22338

runNode3:
	go run main.go --data-dir ./build/node3/chain --enable-consensus --loglevel 1 --networkid 4 --rest --wallet ./build/node3/wallet/wallet.dat --rpcport 23336 --rest --restport 23334 --consensus-port 23339 --nodeport 23338

runNode4:
	go run main.go  --data-dir ./build/node4/chain --enable-consensus --loglevel 1 --networkid 4 --rest --wallet ./build/node4/wallet/wallet.dat --rpcport 24336 --rest --restport 24334 --consensus-port 24339 --nodeport 24338

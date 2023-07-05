.PHONY: server
server: wasm
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o limsrv github.com/NanoRed/lim/cmd/server
	scp ./limsrv red@$(SERVER_IP):~/limsrv
	rm -f ./limsrv

.PHONY: client
client:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o limcli.exe github.com/NanoRed/lim/cmd/client
	mv ./limcli.exe /mnt/d/mine/lim

.PHONY: wasm
wasm:
	sed -i 's/wss:\/\/127.0.0.1:7715\//wss:\/\/$(SERVER_DOMAIN):$(SERVER_WS_PORT)\//g' ./cmd/wasm/main.go
	tinygo build -o ./website/chatroom/wasm/limcli.wasm -target wasm github.com/NanoRed/lim/cmd/wasm
	sed -i 's/wss:\/\/$(SERVER_DOMAIN):$(SERVER_WS_PORT)\//wss:\/\/127.0.0.1:7715\//g' ./cmd/wasm/main.go

.PHONY: turn
turn:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o limturn github.com/NanoRed/lim/cmd/turn
	scp ./limturn red@$(SERVER_IP):~/limturn
	rm -f ./limturn
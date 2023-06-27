.PHONY: server
server:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o limsrv github.com/NanoRed/lim/cmd/server
	scp ./limsrv red@$(SERVER_IP):~/limsrv
	rm -f ./limsrv

.PHONY: client
client:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o limcli.exe github.com/NanoRed/lim/cmd/client
	mv ./limcli.exe /mnt/d/mine/lim

.PHONY: wasm
wasm:
	sed -i 's/ws:\/\/127.0.0.1:7715\//ws:\/\/$(SERVER_IP):$(SERVER_WS_PORT)\//g' ./cmd/wasm/main.go
	tinygo build -o ./website/chatroom/wasm/limcli.wasm -target wasm github.com/NanoRed/lim/cmd/wasm
	sed -i 's/ws:\/\/$(SERVER_IP):$(SERVER_WS_PORT)\//ws:\/\/127.0.0.1:7715\//g' ./cmd/wasm/main.go
	
.PHONY: website
website: wasm
	GOOS=linux GOARCH=amd64 go build -o limwebsite github.com/NanoRed/lim/cmd/website
	scp ./limwebsite red@$(SERVER_IP):~/limwebsite
	rm -f ./limwebsite
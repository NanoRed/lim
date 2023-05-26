server:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o limsrv_test github.com/NanoRed/lim/cmd/server
	scp ./limsrv_test red@106.52.81.44:~/limsrv_test
	rm -f ./limsrv_test

client:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o limcli.exe github.com/NanoRed/lim/cmd/client
	mv ./limcli.exe /mnt/s/mine/lim

wasm:
	GOOS=js GOARCH=wasm go build -ldflags "-X main.ip=106.52.81.44 -X main.port=8000" -o limcli.wasm github.com/NanoRed/lim/cmd/wasm
server:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o limsrv github.com/NanoRed/lim/cmd/server
	scp ./limsrv red@106.52.81.44:~/limsrv
	rm -f ./limsrv

client:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o limcli.exe github.com/NanoRed/lim/cmd/client
	mv ./limcli.exe /mnt/s/mine/lim
	# CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o limcli github.com/NanoRed/lim/cmd/client
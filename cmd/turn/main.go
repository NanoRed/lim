package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/NanoRed/lim/pkg/logger"
	"github.com/pion/turn/v2"
)

var (
	turnIP    = flag.String("turnIP", "127.0.0.1", "input the turn server IP")
	turnPort  = flag.String("turnPort", "3478", "input the turn server port")
	turnRealm = flag.String("turnRealm", "red", "input the turn server realm")
	turnUsers = flag.String("turnUsers", "red=123456", "input the turn server users")
	publicIP  = flag.String("pubIP", "106.52.81.44", "input the server public IP")
)

func main() {
	flag.Parse()

	udpListener, err := net.ListenPacket("udp4", fmt.Sprintf("%s:%s", *turnIP, *turnPort))
	if err != nil {
		logger.Panic("listen udp4 error: %v", err)
	}
	usersMap := map[string][]byte{}
	for _, kv := range regexp.MustCompile(`(\w+)=(\w+)`).FindAllStringSubmatch(*turnUsers, -1) {
		usersMap[kv[1]] = turn.GenerateAuthKey(kv[1], *turnRealm, kv[2])
	}
	s, err := turn.NewServer(turn.ServerConfig{
		Realm: *turnRealm,
		// Set AuthHandler callback
		// This is called every time a user tries to authenticate with the TURN server
		// Return the key for that user, or false when no user is found
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			if key, ok := usersMap[username]; ok {
				return key, true
			}
			return nil, false
		},
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(*publicIP), // Claim that we are listening on IP passed by user (This should be your Public IP)
					Address:      "0.0.0.0",              // But actually be listening on every interface
				},
			},
		},
	})
	if err != nil {
		logger.Panic("turn server error: %v", err)
	}

	// Block until user sends SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	if err = s.Close(); err != nil {
		logger.Panic("turn server close error: %v", err)
	}
}

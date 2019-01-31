package tester

import (
	"fmt"
	"math/rand"
	_ "os" // required for TODO
	"strconv"
	"strings"
	_ "sync" // required for TODO
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/peers"
	"github.com/Fantom-foundation/go-lachesis/src/proxy"
)

// PingNodesN ping the nodes to make sure they are communicating
func PingNodesN(participants []*peers.Peer, n int, delay uint64, logger *logrus.Logger, ProxyAddr string) {
	// pause before shooting test transactions
	time.Sleep(time.Duration(delay) * time.Second)

	proxies := make(map[common.Address]*proxy.GrpcLachesisProxy)
	for _, participant := range participants {
		if participant.NetAddr == "" {
			fmt.Printf("node missing NetAddr [%v]", participant)
			continue
		}
		hostPort := strings.Split(participant.NetAddr, ":")
		port, err := strconv.Atoi(hostPort[1])
		if err != nil {
			fmt.Printf("error:\t\t\t%s\n", err.Error())
			fmt.Printf("Unable to create port:\t\t\t%s (id=%d)\n", participant.NetAddr, participant.ID)
		}
		addr := fmt.Sprintf("%s:%d", hostPort[0], port-3000 /*9000*/)
		lachesisProxy, err := proxy.NewGrpcLachesisProxy(addr, logger)
		if err != nil {
			fmt.Printf("error:\t\t\t%s\n", err.Error())
			fmt.Printf("Failed to create WebsocketLachesisProxy:\t\t\t%s (id=%d)\n", participant.NetAddr, participant.ID)
		}
		proxies[participant.ID] = lachesisProxy
	}
	for iteration := 0; iteration < n; iteration++ {
		participant := participants[rand.Intn(len(participants))]

		_, err := transact(proxies[participant.ID], ProxyAddr, iteration)

		if err != nil {
			fmt.Printf("error:\t\t\t%s\n", err.Error())
			fmt.Printf("Failed to ping:\t\t\t%s (id=%d)\n", participant.NetAddr, participant.ID)
			fmt.Printf("Failed to send transaction:\t%d\n\n", iteration)
		}
	}

	for _, lachesisProxy := range proxies {
		lachesisProxy.Close()
	}
	fmt.Println("Pinging stopped after ", n, " iterations")
}

func transact(proxy *proxy.GrpcLachesisProxy, proxyAddr string, iteration int) (string, error) {

	// Ethereum txns are ~108 bytes. Bitcoin txns are ~250 bytes.
	// A good assumption is to make txns 120 bytes in size.
	// However, for speed, we're using 1 byte here. Modify accordingly.
	// var msg [1]byte
	for i := 0; i < 10; i++ {
		// Send 10 txns to the server.
		msg := fmt.Sprintf("%s.%d.%d", proxyAddr, iteration, i)
		err := proxy.SubmitTx([]byte(msg))
		if err != nil {
			return "", err
		}
	}
	// fmt.Println("Submitted tx, ack=", ack)  # `ack` is now `_`

	return "", nil
}

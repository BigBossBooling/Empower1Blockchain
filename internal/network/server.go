package network

import (
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
)

func NewServer(port int) (host.Host, error) {
	listenAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
	if err != nil {
		return nil, err
	}

	host, err := libp2p.New(
		libp2p.ListenAddrs(listenAddr),
	)
	if err != nil {
		return nil, err
	}

	log.Printf("Host created with id %s\n", host.ID())
	log.Printf("Listen addresses: %s\n", host.Addrs())

	return host, nil
}

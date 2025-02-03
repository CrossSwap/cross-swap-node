package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/multiformats/go-multiaddr"
)

// Bootstrap nodes are peers with well known addresses that are used to find other peers
var bootstrapPeers = []string{
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
}

// discoveryNamespace is used as the namespace for both DHT and mDNS discovery
const discoveryNamespace = "cross-swap-network"

// mDNS discovery interval
const discoveryInterval = time.Second * 10

// connectedPeers keeps track of peers we're connected to
var connectedPeers = make(map[peer.ID]struct{})
var peerMapLock sync.RWMutex

// mdnsNotifee gets notified when we find a new peer via mDNS discovery
type mdnsNotifee struct {
	h host.Host
}

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// Check if we're already connected
	peerMapLock.RLock()
	_, exists := connectedPeers[pi.ID]
	peerMapLock.RUnlock()

	if exists {
		return
	}

	if n.h.Network().Connectedness(pi.ID) != network.Connected {
		log.Printf("📡 Found new peer via mDNS: %s", pi.ID.String()[:12])
		if err := n.h.Connect(context.Background(), pi); err != nil {
			// Only log if it's not a self-dial or common connection error
			if !strings.Contains(err.Error(), "dial to self attempted") &&
				!strings.Contains(err.Error(), "connection refused") &&
				!strings.Contains(err.Error(), "i/o timeout") {
				log.Printf("❌ Failed to connect to local peer %s: %s", pi.ID.String()[:12], err)
			}
		} else {
			peerMapLock.Lock()
			connectedPeers[pi.ID] = struct{}{}
			peerMapLock.Unlock()
			log.Printf("✅ Connected to local peer: %s", pi.ID.String()[:12])
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up logging
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	log.Println("🚀 Starting Cross-Swap Node...")

	// Create a new libp2p Host that listens on a fixed port
	h, err := libp2p.New(
		// Listen on specific addresses
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/63785", // TCP port
			"/ip6/::/tcp/63785",      // TCP port (IPv6)
		),
		// Enable relay client functionality
		libp2p.EnableRelay(),
	)
	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}
	defer h.Close()

	// Setup Local Network Discovery (mDNS)
	if err := setupMDNS(h, discoveryNamespace); err != nil {
		log.Printf("⚠️ Failed to setup mDNS discovery: %s", err)
	} else {
		log.Println("✅ Local network discovery (mDNS) enabled")
	}

	// Create a Kademlia DHT
	kdht, err := dht.New(ctx, h, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal("Failed to create DHT:", err)
	}

	// Connect to bootstrap nodes
	var wg sync.WaitGroup
	for _, addr := range bootstrapPeers {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			continue
		}

		peerinfo, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			continue
		}

		wg.Add(1)
		go func(pi *peer.AddrInfo) {
			defer wg.Done()
			if err := h.Connect(ctx, *pi); err != nil {
				// Only log if it's not a common connection error
				if !strings.Contains(err.Error(), "connection refused") &&
					!strings.Contains(err.Error(), "i/o timeout") {
					log.Printf("⚠️ Bootstrap node %s unavailable", pi.ID.String()[:12])
				}
			} else {
				peerMapLock.Lock()
				connectedPeers[pi.ID] = struct{}{}
				peerMapLock.Unlock()
				log.Printf("✅ Connected to bootstrap node: %s", pi.ID.String()[:12])
			}
		}(peerinfo)
	}
	wg.Wait()

	// Bootstrap the DHT
	if err = kdht.Bootstrap(ctx); err != nil {
		log.Fatal("Failed to bootstrap DHT:", err)
	}

	// Print the node's detailed information
	log.Println("\n📋 Node Information:")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("🆔 Node ID: %s\n", h.ID().String()[:12])
	log.Println("📡 Listening Addresses:")

	// Only show relevant addresses
	for _, addr := range h.Addrs() {
		addrStr := addr.String()
		if strings.Contains(addrStr, "127.0.0.1") ||
			strings.Contains(addrStr, "::1") {
			continue // Skip localhost addresses
		}
		fullAddr := addr.Encapsulate(multiaddr.StringCast("/p2p/" + h.ID().String()))
		log.Printf("   └─ %s\n", fullAddr)
	}
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Set up peer discovery
	routingDiscovery := drouting.NewRoutingDiscovery(kdht)

	// Advertise this node
	routingDiscovery.Advertise(ctx, discoveryNamespace)
	log.Println("✨ Successfully announced ourselves to the network")

	// Look for other peers
	go discoverPeers(ctx, routingDiscovery, h)

	log.Println("✅ Node is running! Press Ctrl+C to stop.")

	// Wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Println("Received signal, shutting down...")
}

func setupMDNS(h host.Host, ns string) error {
	s := mdns.NewMdnsService(h, ns, &mdnsNotifee{h: h})
	return s.Start()
}

func discoverPeers(ctx context.Context, routingDiscovery *drouting.RoutingDiscovery, h host.Host) {
	ticker := time.NewTicker(discoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			peers, err := routingDiscovery.FindPeers(ctx, discoveryNamespace)
			if err != nil {
				continue
			}

			for peer := range peers {
				// Skip if it's us or if we're already connected
				if peer.ID == h.ID() {
					continue
				}

				peerMapLock.RLock()
				_, exists := connectedPeers[peer.ID]
				peerMapLock.RUnlock()
				if exists {
					continue
				}

				if h.Network().Connectedness(peer.ID) != network.Connected {
					_, err = h.Network().DialPeer(ctx, peer.ID)
					if err != nil {
						// Only log if it's not a common connection error
						if !strings.Contains(err.Error(), "dial to self attempted") &&
							!strings.Contains(err.Error(), "connection refused") &&
							!strings.Contains(err.Error(), "i/o timeout") {
							log.Printf("❌ Failed to connect to peer %s: %s\n", peer.ID.String()[:12], err)
						}
						continue
					}
					peerMapLock.Lock()
					connectedPeers[peer.ID] = struct{}{}
					peerMapLock.Unlock()
					log.Printf("🔗 Connected to peer: %s\n", peer.ID.String()[:12])
				}
			}
		}
	}
}

// setupDiscovery sets up peer discovery mechanisms
func setupDiscovery(ctx context.Context, h host.Host, peers chan peer.AddrInfo) {
	// TODO: Implement peer discovery mechanisms
	// This could include:
	// - DHT for peer discovery
	// - mDNS for local network discovery
	// - Static bootstrapping nodes
}

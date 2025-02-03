# Cross-Swap Node

A peer-to-peer node implementation for the Cross-Swap network using libp2p. This node forms part of the decentralized network that enables cross-chain swaps.

## Features

- P2P networking using libp2p
- Kademlia DHT for peer discovery
- Automatic peer discovery and connection
- NAT traversal with relay support
- IPv4 and IPv6 support

## Requirements

- Go 1.22 or later

## Installation

1. Clone the repository:
```bash
git clone https://github.com/CrossSwap/cross-swap-node.git
cd cross-swap-node
```

2. Install dependencies:
```bash
go mod tidy
```

## Running a Node

To start a node:

```bash
go run main.go
```

The node will:
1. Generate a unique peer ID
2. Listen on port 63785 (TCP)
3. Connect to bootstrap nodes
4. Start discovering other Cross-Swap nodes
5. Automatically connect to discovered peers

## Network Information

- Default Port: 63785 (TCP)
- Network ID: "cross-swap-network"
- Discovery: Kademlia DHT with bootstrap nodes

## Development

The node is built using:
- `go-libp2p` for P2P networking
- `go-libp2p-kad-dht` for peer discovery
- `go-multiaddr` for network addressing

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

TBD

## Contact

Project Link: [https://github.com/CrossSwap/cross-swap-node](https://github.com/CrossSwap/cross-swap-node) 
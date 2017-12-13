package p2p2

import (
	"github.com/UnrulyOS/go-unruly/log"
	"github.com/UnrulyOS/go-unruly/p2p2/pb"
	"github.com/gogo/protobuf/proto"
)

// Swarm
// Hold ref to peers and manage connections to peers

// p2p2 Stack:
// -----------
// Local Node
// -- Implements p2p protocols: impls with req/resp support (match id resp w request id)
//	 -- ping protocol
//   -- echo protocol
//   -- all other protocols...
// DeMuxer (node aspect) - routes remote requests (and responses) back to protocols. Protocols register on muxer
// Swarm forward incoming messages to muxer. Let protocol send message to a remote node. Connects to random nodes.
// Swarm - managed all remote nodes, sessions and connections
// Remote Node - maintains sessions - should be internal to swarm only. Remote node id str only above this
// -- NetworkSession (optional)
// Connection
// -- msgio (prefix length encoding). shared buffers.
// Network
//	- tcp for now
//  - utp / upd soon

type Swarm interface {

	// Attempt to establish a session with a remote node - useful for bootstraping
	ConnectTo(req NodeReq)

	// ConnectToNodes(maxNodes int) Get random nodes (max int) get up to max random nodes from the swarm

	// forcefully disconnect form a node
	DisconnectFrom(req NodeReq)

	// Send a message to a remote node
	SendMessage(req SendMessageReq)

	// todo: Register muxer to handle incoming messages to higher level protocols and handshake protocol

	GetDemuxer() Demuxer

	LocalNode() LocalNode
}

type SendMessageReq struct {
	remoteNodeId string // string encoded key
	reqId        string
	msg          []byte
}

// client node conn req data
type NodeReq struct {
	remoteNodeId string
	remoteNodeIp string
	callback     chan NodeResp
}

type NodeResp struct {
	remoteNodeId string
	err          error
}

type swarmImpl struct {

	// set in construction and immutable state
	network   Network
	localNode LocalNode
	demuxer   Demuxer

	// all data should only be accessed from methods executed by the main swarm event loop

	// Internal state not thread safe state - must be access only from methods dispatched from the internal event handler
	peers             map[string]RemoteNode // remote known nodes mapped by their ids (keys) - Swarm is a peer store. NodeId -> RemoteNode
	connections       map[string]Connection // all open connections to nodes by conn id. ConnId -> Con.
	peersByConnection map[string]RemoteNode // remote nodes indexed by their connections. ConnId -> RemoteNode

	pendingOutgoingMessages map[string]SendMessageReq // all messages we don't have a response for yet. k - reqId

	// add registered callbacks in a sync.map to return to the muxer responses to outgoing messages

	// comm channels
	connectionRequests chan NodeReq        // request to establish a session w a remote node
	disconnectRequests chan NodeReq        // kill session and disconnect from node
	sendMsgRequests    chan SendMessageReq // send a message to a node and callback on error or data
	kill               chan bool           // used to kill the swamp from outside. e.g when local node is shutting down

}

func NewSwarm(tcpAddress string, l LocalNode) (Swarm, error) {

	n, err := NewNetwork(tcpAddress)
	if err != nil {
		log.Error("can't create swarm without a network: %v", err)
		return nil, err
	}

	s := &swarmImpl{
		localNode:               l,
		network:                 n,
		kill:                    make(chan bool),
		peersByConnection:       make(map[string]RemoteNode),
		peers:                   make(map[string]RemoteNode),
		connections:             make(map[string]Connection),
		pendingOutgoingMessages: make(map[string]SendMessageReq),
		connectionRequests:      make(chan NodeReq, 10),
		disconnectRequests:      make(chan NodeReq, 10),
		sendMsgRequests:         make(chan SendMessageReq, 20),
		demuxer:                 NewDemuxer(),
	}

	go s.beginProcessingEvents()

	return s, err
}

func (s *swarmImpl) ConnectTo(req NodeReq) {
	s.connectionRequests <- req
}

func (s *swarmImpl) DisconnectFrom(req NodeReq) {
	s.disconnectRequests <- req
}

func (s *swarmImpl) GetDemuxer() Demuxer {
	return s.demuxer
}

func (s *swarmImpl) LocalNode() LocalNode {
	return s.localNode
}

// Send a message to a remote node
// Swarm will establish session if needed or use an existing session and open connection
// Designed to be used by any high level protocol
// req.reqId: globally unique id string - used for tracking messages we didn't get a response for yet
// req.msg: marshaled message data
// req.destId: receiver remote node public key/id

func (s *swarmImpl) SendMessage(req SendMessageReq) {
	s.sendMsgRequests <- req
}

// Swarm serial event processing
// provides concurrency safety as only one callback is executed at a time
// so there's no need for sync internal data structures
func (s *swarmImpl) beginProcessingEvents() {

Loop:
	for {
		select {
		case <-s.kill:
			// todo: gracefully stop the swarm - close all connections to remote nodes
			break Loop

		case c := <-s.network.GetNewConnections():
			s.onRemoteClientConnected(c)

		case m := <-s.network.GetIncomingMessage():
			s.onRemoteClientMessage(m)

		case err := <-s.network.GetConnectionErrors():
			s.onConnectionError(err)

		case err := <-s.network.GetMessageSendErrors():
			s.onMessageSendError(err)

		case c := <-s.network.GetClosingConnections():
			s.onConnectionClosed(c)

		case r := <-s.sendMsgRequests:
			s.onSendMessageRequest(r)

		case n := <-s.connectionRequests:
			s.onConnectionRequest(n)

		case n := <-s.disconnectRequests:
			s.onDisconnectionRequest(n)
		}
	}
}

// connect to node or ensure we are connected
// start a session on demand if needed
func (s *swarmImpl) onConnectionRequest(req NodeReq) {

	// check for existing session
	remoteNode := s.peers[req.remoteNodeId]

	if remoteNode == nil {

		remoteNode, err := NewRemoteNode(req.remoteNodeId, req.remoteNodeIp)
		if err != nil {
			return
		}

		// store new remote node by id
		s.peers[req.remoteNodeId] = remoteNode
	}

	if remoteNode != nil && remoteNode.HasSession() {
		log.Info("We have a session with this node")

		remoteNode.GetSession(func(s NetworkSession) {
			log.Info("Session info: %s", s.IsAuthenticated())
		})

		return
	}

	// start handshake protocol
	s.localNode.HandshakeProtocol().CreateSession(remoteNode)
}

func (s *swarmImpl) onDisconnectionRequest(req NodeReq) {
	// disconnect from node...
}


func (s *swarmImpl) onSendMessageRequest(r SendMessageReq) {

	// check for existing session
	//remoteNode := s.peers[r.remoteNodeId]


	// todo: send message here - establish a connection and session on-demand as needed
	// todo: auto support for retries

	// req ids are unique - store as pending until we get a response, error or timeout
	s.pendingOutgoingMessages[r.reqId] = r
}

func (s *swarmImpl) onConnectionClosed(c Connection) {
	delete(s.connections, c.Id())
	delete(s.peersByConnection, c.Id())
}

func (s *swarmImpl) onRemoteClientConnected(c Connection) {
	// nop - a remote client connected - this is handled w message
}

// Main network messages handler
// c: connection we got this message on
// msg: binary protobufs encoded data
// not go safe - called from event processing main loop
func (s *swarmImpl) onRemoteClientMessage(msg ConnectionMessage) {

	// Processing a remote incoming message:

	c := &pb.CommonMessageData{}
	err := proto.Unmarshal(msg.Message, c)
	if err != nil {
		log.Warning("Bad request - closing connection...")
		msg.Connection.Close()
		return
	}

	if len(c.Payload) == 0 {
		// this a handshake protocol message
		// send to muxer (protocol, msg, etc....) - protocol handler will create remote node, session, etc...
	} else {

		// A session encrypted protocol message is in payload

		// 1. find remote node - bail if we can't find it - it should be created on session start

		// 2. get session from remote node - if session not found close the connection

		// attempt to decrypt message (c.payload) with active session key

		// create pb.ProtocolMessage from the decrypted message

		// send to muxer pb.protocolMessage to the muxer for handling - it has protocol, reqId, sessionId, etc....
	}

	//data := proto.Unmarshal(msg.Message, proto.Message)

	// 1. decyrpt protobuf to a generic protobuf obj - all messages are protobufs but we don't know struct type just yet

	// there are 2 types of high-level payloads: session establishment handshake (req or resp)
	// and messages encrypted using session id

	// 2. if session id is included in message and connection has this key then use it to decrypt payload - other reject and close conn

	// 3. if req is a handshake request or response then handle it to establish a session - use code here or use handshake protocol handler

	// 4. if req is from a remote node we don't know about so create it

	// 5. save connection for this remote node - one connection per node for now

	// 6. auth message sent by remote node pub key close connection otherwise

	// Locate callback a pendingOutgoingMessages for the req id then push the resp over the embedded channel callback to notify caller
	// of the response data - muxer will forward it to handlers and handler can create struct from buff based on expected typed data
}

// not go safe - called from event processing main loop
func (s *swarmImpl) onConnectionError(err ConnectionError) {
	// close the connection?
	// who to notify?
}

// not go safe - called from event processing main loop
func (s *swarmImpl) onMessageSendError(err MessageSendError) {
	// retry ?
}

// todo: handshake protocol

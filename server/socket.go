package server

import (
	"encoding/json"
	_ "fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/nack"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
	"log"
	"net/http"
	"sync"
)

var (
	SocketConnections []Connection
	peerConnections   = make(map[string]P)
)

type P struct {
	peerConnection map[string]*webrtc.PeerConnection
}

type Message struct {
	PeerType string      `json:"peerType"`
	Type     string      `json:"type"`
	Data     interface{} `json:"data"`
}

type Connection struct {
	ClientId         string          `json:"clientId"`
	SocketConnection *websocket.Conn `json:"socketConnection"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type SafeWebSocket struct {
	Connection *websocket.Conn `json:"connection"`
	Mutex      sync.Mutex      `json:"mutex"`
}

var iceServers = []webrtc.ICEServer{
	{
		URLs: []string{"stun:stun.l.google.com:19302"},
	},
}

var config = webrtc.Configuration{
	ICEServers: iceServers,
}

func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	connection, _ := upgrader.Upgrade(w, r, nil)

	clientId := uuid.New().String()
	conn := Connection{ClientId: clientId, SocketConnection: connection}

	SocketConnections = append(SocketConnections, conn)

	defer func() {
		err := connection.Close()
		if err != nil {
			return
		}
	}()

	for {
		_, message, err1 := connection.ReadMessage()
		if err1 != nil {
			break
		}

		var data Message
		err2 := json.Unmarshal(message, &data)
		if err2 != nil {
			break
		}

		switch data.Type {
		case "offer":
			{
				HandleOffer(data, clientId)
			}
		case "answer":
			{
				HandleAnswer(data, clientId)
			}
		case "candidate":
			{
				HandleCandidate(data, clientId)
			}
		}
	}
}

func HandleOffer(data Message, clientId string) {

	connection := GetSocketConnection(clientId)
	safeConnection := &SafeWebSocket{
		Connection: connection,
	}

	senderPeer, _ := getPeerConnection()
	receiverPeer, _ := getPeerConnection()

	audioTrackLocal, _ := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion")
	videoTrackLocal, _ := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")

	transceiverInitRecevOnly := webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}
	transceiverInitSendOnly := webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	}

	_, _ = receiverPeer.AddTransceiverFromTrack(audioTrackLocal, transceiverInitRecevOnly)
	_, _ = receiverPeer.AddTransceiverFromTrack(videoTrackLocal, transceiverInitRecevOnly)

	for i := 0; i < 1; i++ {
		trackLocal, _ := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, uuid.NewString(), uuid.NewString())
		_, _ = senderPeer.AddTransceiverFromTrack(trackLocal, transceiverInitSendOnly)
	}

	for i := 0; i < 1; i++ {
		trackLocal, _ := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, uuid.NewString(), uuid.NewString())
		_, _ = senderPeer.AddTransceiverFromTrack(trackLocal, transceiverInitSendOnly)
	}

	receiverPeer.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}

		candidateMessage := Message{
			PeerType: "receiver",
			Type:     "candidate",
			Data:     i.ToJSON(),
		}

		safeConnection.Mutex.Lock()
		_ = connection.WriteJSON(candidateMessage)
		safeConnection.Mutex.Unlock()
	})

	senderPeer.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}

		candidateMessage := Message{
			PeerType: "sender",
			Type:     "candidate",
			Data:     i.ToJSON(),
		}

		safeConnection.Mutex.Lock()
		_ = connection.WriteJSON(candidateMessage)
		safeConnection.Mutex.Unlock()
	})

	receiverPeer.OnConnectionStateChange(func(p webrtc.PeerConnectionState) {
		switch p {
		case webrtc.PeerConnectionStateFailed:
			log.Printf(Red+"ID: %s Receiver Peer connection failed"+Reset, clientId)
		case webrtc.PeerConnectionStateClosed:
			log.Printf(Red+"ID: %s Receiver Peer connection closed"+Reset, clientId)
		case webrtc.PeerConnectionStateDisconnected:
			log.Printf(Red+"ID: %s Receiver Peer connection disconnected"+Reset, clientId)
		case webrtc.PeerConnectionStateConnected:
			log.Printf(Green+"ID: %s Receiver Peer connection connected"+Reset, clientId)
		default:
			//log.Println("Default")
		}
	})

	receiverPeer.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		switch state {
		case webrtc.ICEConnectionStateNew:
			log.Printf("ID: %s Receiver Peer ICE agent waiting for remote candidates.", clientId)
		case webrtc.ICEConnectionStateConnected:
			log.Printf(Green+"ID: %s Receiver Peer ICE agent connected."+Reset, clientId)
		case webrtc.ICEConnectionStateCompleted:
			log.Printf(Green+"ID: %s Receiver Peer ICE agent completed."+Reset, clientId)
		case webrtc.ICEConnectionStateFailed:
			log.Printf(Red+"ID: %s Receiver Peer ICE agent failed."+Reset, clientId)
		case webrtc.ICEConnectionStateDisconnected:
			log.Printf(Red+"ID: %s Receiver Peer ICE agent disconnected."+Reset, clientId)
		case webrtc.ICEConnectionStateClosed:
			log.Printf(Red+"ID: %s Receiver Peer ICE agent closed."+Reset, clientId)
		default:
			//log.Printf("Receiver Peer Unknown ICE connection state")
		}
	})

	senderPeer.OnConnectionStateChange(func(p webrtc.PeerConnectionState) {
		switch p {
		case webrtc.PeerConnectionStateFailed:
			log.Printf(Red+"ID: %s Sender Peer connection failed"+Reset, clientId)
		case webrtc.PeerConnectionStateClosed:
			log.Printf(Red+"ID: %s Sender Peer connection closed"+Reset, clientId)
		case webrtc.PeerConnectionStateDisconnected:
			log.Printf(Red+"ID: %s Sender Peer connection disconnected"+Reset, clientId)
		case webrtc.PeerConnectionStateConnected:
			log.Printf(Green+"ID: %s Sender Peer connection connected"+Reset, clientId)
		default:
			//log.Println("Default")
		}
	})

	senderPeer.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		switch state {
		case webrtc.ICEConnectionStateNew:
			log.Printf("ID: %s Sender Peer ICE agent waiting for remote candidates.", clientId)
		case webrtc.ICEConnectionStateConnected:
			log.Printf(Green+"ID: %s Sender Peer ICE agent connected."+Reset, clientId)
		case webrtc.ICEConnectionStateCompleted:
			log.Printf(Green+"ID: %s Sender Peer ICE agent completed."+Reset, clientId)
		case webrtc.ICEConnectionStateFailed:
			log.Printf(Red+"ID: %s Sender Peer ICE agent failed."+Reset, clientId)
		case webrtc.ICEConnectionStateDisconnected:
			log.Printf(Red+"ID: %s Sender Peer ICE agent disconnected."+Reset, clientId)
		case webrtc.ICEConnectionStateClosed:
			log.Printf(Red+"ID: %s Sender Peer ICE agent closed."+Reset, clientId)
		default:
			//log.Printf("Sender Peer Unknown ICE connection state")
		}
	})

	receiverPeer.OnTrack(func(t *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		go func() {
			for {
				_, _, err := receiver.ReadRTCP()
				if err != nil {
					log.Printf(Red+"ID: %s Error reading RTCP: "+Reset+"%v", clientId, err)
					break
				}
			}
		}()

		buf := make([]byte, 1400)
		log.Printf(Blue+"ID: %s Track Received: Kind: "+Reset+"%s", clientId, t.Kind())
		for {
			_, _, err := t.Read(buf)
			if err != nil {
				log.Printf(Red+"ID: %s Error reading track: "+Reset+"%v", clientId, err)
				break
			}
		}
	})

	senderOffer, _ := senderPeer.CreateOffer(nil)
	_ = senderPeer.SetLocalDescription(senderOffer)

	offerMessage := Message{
		PeerType: "sender",
		Type:     "offer",
		Data:     senderOffer,
	}

	safeConnection.Mutex.Lock()
	_ = connection.WriteJSON(offerMessage)
	safeConnection.Mutex.Unlock()

	var offer webrtc.SessionDescription
	_ = json.Unmarshal([]byte(data.Data.(string)), &offer)
	_ = receiverPeer.SetRemoteDescription(offer)
	answer, _ := receiverPeer.CreateAnswer(nil)
	_ = receiverPeer.SetLocalDescription(answer)

	answerMessage := Message{
		PeerType: "receiver",
		Type:     "answer",
		Data:     answer,
	}

	safeConnection.Mutex.Lock()
	_ = connection.WriteJSON(answerMessage)
	safeConnection.Mutex.Unlock()

	pc := make(map[string]*webrtc.PeerConnection)
	pc["sender"] = senderPeer
	pc["receiver"] = receiverPeer
	peerConnections[clientId] = P{peerConnection: pc}
}

func HandleAnswer(data Message, clientId string) {
	peerCons, peerConsEx := peerConnections[clientId]
	if peerConsEx {
		senderPeer, senderPeerEx := peerCons.peerConnection["sender"]
		if senderPeerEx {
			var answer webrtc.SessionDescription
			_ = json.Unmarshal([]byte(data.Data.(string)), &answer)
			_ = senderPeer.SetRemoteDescription(answer)
		}
	}
}

func HandleCandidate(data Message, clientId string) {
	if data.PeerType == "receiver" {
		peerCons, peerConsEx := peerConnections[clientId]
		if peerConsEx {
			senderPeer, senderPeerEx := peerCons.peerConnection["sender"]
			if senderPeerEx {
				var candidate webrtc.ICECandidateInit
				_ = json.Unmarshal([]byte(data.Data.(string)), &candidate)
				_ = senderPeer.AddICECandidate(candidate)
			}
		}
	} else if data.PeerType == "sender" {
		peerCons, peerConsEx := peerConnections[clientId]
		if peerConsEx {
			receiverPeer, receiverPeerEx := peerCons.peerConnection["receiver"]
			if receiverPeerEx {
				var candidate webrtc.ICECandidateInit
				_ = json.Unmarshal([]byte(data.Data.(string)), &candidate)
				_ = receiverPeer.AddICECandidate(candidate)
			}
		}
	}
}

func getPeerConnection() (*webrtc.PeerConnection, error) {
	mediaEngine := &webrtc.MediaEngine{}
	err := mediaEngine.RegisterDefaultCodecs()

	if err != nil {
		log.Fatalf("Failed to register default codecs %s", err)
	}
	interceptorRegistry := &interceptor.Registry{}

	ips := []string{"192.168.2.107"}
	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetNAT1To1IPs(ips, webrtc.ICECandidateTypeHost)
	//settingEngine.DisableMediaEngineCopy(true)
	//settingEngine.SetDTLSEllipticCurves(elliptic.X25519, elliptic.P384, elliptic.P256)
	//settingEngine.SetICETimeouts(
	//	time.Second*5,  // disconnectedTimeout: Timeout for considering the connection disconnected
	//	time.Second*10, // failedTimeout: Timeout for considering the connection failed
	//	time.Second*15, // keepAliveInterval: Interval for sending ICE keep-alive messages
	//)
	err = settingEngine.SetEphemeralUDPPortRange(50000, 60000)
	if err != nil {
		log.Fatalf("Failed to set udp port range: %s", err)
	}

	err = RegisterDefaultInterceptors(mediaEngine, interceptorRegistry)
	if err != nil {
		log.Fatalf("Failed to register default interceptors %s", err)
	}
	err = mediaEngine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: sdp.AudioLevelURI}, webrtc.RTPCodecTypeAudio)
	if err != nil {
		log.Fatalf("Failed to register header extension %s", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry), webrtc.WithSettingEngine(settingEngine))

	return api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
					"stun:stun1.l.google.com:19302",
					"stun:stun2.l.google.com:19302",
					"stun:stun3.l.google.com:19302",
					"stun:stun4.l.google.com:19302",
				},
			},
		},
	})
}

func RegisterDefaultInterceptors(mediaEngine *webrtc.MediaEngine, interceptorRegistry *interceptor.Registry) error {
	if err := ConfigureNack(mediaEngine, interceptorRegistry); err != nil {
		return err
	}

	if err := webrtc.ConfigureRTCPReports(interceptorRegistry); err != nil {
		return err
	}

	if err := webrtc.ConfigureSimulcastExtensionHeaders(mediaEngine); err != nil {
		return err
	}

	return webrtc.ConfigureTWCCSender(mediaEngine, interceptorRegistry)
}

func ConfigureNack(mediaEngine *webrtc.MediaEngine, interceptorRegistry *interceptor.Registry) error {
	generator, err := nack.NewGeneratorInterceptor()
	if err != nil {
		return err
	}

	responder, err := nack.NewResponderInterceptor(nack.DisableCopy())
	if err != nil {
		return err
	}

	mediaEngine.RegisterFeedback(webrtc.RTCPFeedback{Type: "nack"}, webrtc.RTPCodecTypeVideo)
	mediaEngine.RegisterFeedback(webrtc.RTCPFeedback{Type: "nack", Parameter: "pli"}, webrtc.RTPCodecTypeVideo)
	interceptorRegistry.Add(responder)
	interceptorRegistry.Add(generator)
	return nil
}

func GetSocketConnection(clientId string) *websocket.Conn {
	for _, conn := range SocketConnections {
		if conn.ClientId == clientId {
			return conn.SocketConnection
		}
	}
	return nil
}

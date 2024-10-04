let localStream;
let receiverPeer;
let senderPeer;
let protocol = 'ws'
let socketPort = 3000
let socketHost = '192.168.2.107'
const ws = new WebSocket(`${protocol}://${socketHost}:${socketPort}/websocket`);
let configuration = {
    iceServers: [{urls: "stun:stun.l.google.com:19302"}],
};

const createPeerConnection = async () => {
    receiverPeer = new RTCPeerConnection(configuration);
    senderPeer = new RTCPeerConnection(configuration);

    receiverPeer.addEventListener("icecandidate", (event) => {
        if (event.candidate) {
            ws.send(JSON.stringify({
                peerType: "receiver", type: "candidate", data: JSON.stringify(event.candidate),
            }));
        }
    });

    senderPeer.addEventListener("icecandidate", (event) => {
        if (event.candidate) {
            ws.send(JSON.stringify({
                peerType: "sender", type: "candidate", data: JSON.stringify(event.candidate),
            }));
        }
    });

    senderPeer.addEventListener(
        "connectionstatechange",
        (event) => {
            switch (senderPeer.connectionState) {
                case "new":
                case "connecting":
                    console.log("Sender Peer Connecting...");
                    break;
                case "connected":
                    console.log("Sender Peer Connected...");
                    break;
                case "disconnected":
                    console.log("Sender Peer Disconnecting...");
                    break;
                case "closed":
                    console.log("Sender Peer Closed...");
                    break;
                case "failed":
                    console.log("Sender Peer Failed...");
                    break;
                default:
                    console.log("Sender Peer Unknown...");
                    break;
            }
        },
        false,
    );

    receiverPeer.addEventListener(
        "connectionstatechange",
        (event) => {
            switch (receiverPeer.connectionState) {
                case "new":
                case "connecting":
                    console.log("Receiver Peer Connecting...");
                    break;
                case "connected":
                    console.log("Receiver Peer Connected...");
                    break;
                case "disconnected":
                    console.log("Receiver Peer Disconnecting...");
                    break;
                case "closed":
                    console.log("Receiver Peer Closed...");
                    break;
                case "failed":
                    console.log("Receiver Peer Failed...");
                    break;
                default:
                    console.log("Receiver Peer Unknown...");
                    break;
            }
        },
        false,
    );

    receiverPeer.addEventListener("track", (event) => {
        if (event.track.kind === "video") {
           console.log("Video Track Received....")
        } else if (event.track.kind === "audio") {
            console.log("Audio Track Received....")
        }
    });
};


const init = async () => {
    await createPeerConnection();
    localStream = await navigator.mediaDevices.getUserMedia({
        video: true, audio: true,
    });

    localStream.getTracks().forEach((track) => {
        senderPeer.addTrack(track, localStream);
    });

    if (senderPeer) {
        const offer = await senderPeer.createOffer();
        await senderPeer.setLocalDescription(offer);
        ws.send(JSON.stringify({peerType: "sender", type: "offer", data: JSON.stringify(offer)}));
    }
};

ws.onmessage = async (message) => {
    const msg = JSON.parse(message.data);

    switch (msg.type) {
        case "offer": {
            await handleOffer(msg);
            break;
        }
        case "answer": {
            await handleAnswer(msg);
            break;
        }
        case "candidate": {
            await handleCandidate(msg);
            break;
        }
    }
};

const handleAnswer = async (message) => {
    const answer = message.data;
    if (senderPeer) {
        await senderPeer.setRemoteDescription(answer)
    }
}

const handleCandidate = async (message) => {
    const candidate = message.data;
    if (receiverPeer) {
        await receiverPeer.addIceCandidate(new RTCIceCandidate(candidate));
    }
};

const handleOffer = async (message) => {
    const offer = message.data;
    if (receiverPeer) {
        await receiverPeer.setRemoteDescription(offer);
        const answer = await receiverPeer.createAnswer();
        await receiverPeer.setLocalDescription(answer);
        ws.send(JSON.stringify({peerType: "receiver", type: "answer", data: JSON.stringify(answer)}));
    }
};

init().then(() => {
    console.log("Init.................")
});


function getRandomColor() {
    const letters = '0123456789ABCDEF';
    let color = '#';
    for (let i = 0; i < 6; i++) {
        color += letters[Math.floor(Math.random() * 16)];
    }
    return color;
}
package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/teris-io/shortid"
	"github.com/vmihailenco/msgpack"
)

type SignalrServiceHandshake struct {
	ServiceUrl string `json:"serviceUrl"`
	JwtBearer  string `json:"accessToken"`
}

type SignalRTransport struct {
	Transport       string   `json:"transport"`
	TransferFormats []string `json:"transferFormats"`
}

type SignalrServerNegotiation struct {
	URL                 string             `json:"url"`
	AccessToken         string             `json:"accessToken"`
	AvailableTransports []SignalRTransport `json:"availableTransports"`
}

type SignalrServiceNegotiation struct {
	ConnectionID        string             `json:"connectionId"`
	AvailableTransports []SignalRTransport `json:"availableTransports"`
}

type SignalrCoreCommon struct {
	WithCounter
	WithSessions
}

func (s *SignalrCoreCommon) SignalrCoreBaseConnect(protocol string) (session *Session, err error) {
	defer func() {
		if err != nil {
			s.counter.Stat("connection:inprogress", -1)
			s.counter.Stat("connection:error", 1)
		}
	}()

	id, err := shortid.Generate()
	if err != nil {
		log.Println("ERROR: failed to generate uid due to", err)
		return
	}

	s.counter.Stat("connection:inprogress", 1)
	wsURL := "ws://" + s.host + "/chat"
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		s.LogError("connection:error", id, "Failed to connect to websocket", err)
		return nil, err
	}

	session = NewSession(id, s.received, s.counter, c)
	if session != nil {
		s.counter.Stat("connection:inprogress", -1)
		s.counter.Stat("connection:established", 1)

		session.Start()
		session.NegotiateProtocol(protocol)
		return
	}

	err = fmt.Errorf("Nil session")
	return
}

func (s *SignalrCoreCommon) SignalrCoreJsonConnect() (*Session, error) {
	return s.SignalrCoreBaseConnect("json")
}

func (s *SignalrCoreCommon) SignalrCoreMsgPackConnect() (session *Session, err error) {
	return s.SignalrCoreBaseConnect("messagepack")
}

func (s *SignalrCoreCommon) SignalrServiceBaseConnect(protocol string) (session *Session, err error) {
	defer func() {
		if err != nil {
			s.counter.Stat("connection:inprogress", -1)
			s.counter.Stat("connection:error", 1)
		}
	}()

	s.counter.Stat("connection:inprogress", 1)

	id, err := shortid.Generate()
	if err != nil {
		log.Println("ERROR: failed to generate uid due to", err)
		return
	}

	negotiateResponse, err := http.Post("http://"+s.host+"/chat/negotiate", "text/plain;charset=UTF-8", strings.NewReader(""))
	if err != nil {
		s.LogError("connection:error", id, "Failed to negotiate with the server", err)
		return
	}
	defer negotiateResponse.Body.Close()

	decoder := json.NewDecoder(negotiateResponse.Body)
	var negotiation SignalrServerNegotiation
	err = decoder.Decode(&negotiation)
	if err != nil {
		s.LogError("connection:error", id, "Failed to decode server negotiation response", err)
		return
	}

	u, err := url.Parse(negotiation.URL)
	if err != nil {
		s.LogError("connection:error", id, "Failed to decode service URL", err)
		return
	}

	u.Path = u.Path + "/negotiate"
	req, err := http.NewRequest("POST", u.String(), strings.NewReader(""))
	if err != nil {
		s.LogError("connection:error", id, "Failed to create POST request to SignalR service", err)
		return
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+negotiation.AccessToken)

	serviceNegotiationResponse, err := http.DefaultClient.Do(req)
	if err != nil {
		s.LogError("connection:error", id, "Failed to post negotiate request to SignalR service", err)
		return
	}
	defer serviceNegotiationResponse.Body.Close()

	decoder = json.NewDecoder(serviceNegotiationResponse.Body)
	var serviceNegotiation SignalrServiceNegotiation
	err = decoder.Decode(&serviceNegotiation)
	if err != nil {
		s.LogError("connection:error", id, "Failed to decode service negotiation response", err)
		return
	}

	u, _ = url.Parse(negotiation.URL)
	query := u.Query()
	query.Set("access_token", negotiation.AccessToken)
	query.Set("id", serviceNegotiation.ConnectionID)
	u.RawQuery = query.Encode()
	u.Scheme = "wss"

	wsURL := u.String()

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		s.LogError("connection:error", id, "Failed to connect to websocket", err)
		return
	}
	session = NewSession(id, s.received, s.counter, c)
	if session != nil {
		s.counter.Stat("connection:inprogress", -1)
		s.counter.Stat("connection:established", 1)

		session.Start()
		session.NegotiateProtocol(protocol)
		return
	}

	err = fmt.Errorf("Nil session")
	return
}

func (s *SignalrCoreCommon) SignalrServiceJsonConnect() (session *Session, err error) {
	return s.SignalrServiceBaseConnect("json")
}

func (s *SignalrCoreCommon) SignalrServiceMsgPackConnect() (session *Session, err error) {
	return s.SignalrServiceBaseConnect("messagepack")
}

var numBitsToShift = []uint{0, 7, 14, 21, 28}

func (s *SignalrCoreCommon) ParseBinaryMessage(bytes []byte) ([]byte, error) {
	moreBytes := true
	msgLen := 0
	numBytes := 0
	//fmt.Printf("%x %x\n", bytes[0], bytes[1])
	for moreBytes && numBytes < len(bytes) && numBytes < 5 {
		byteRead := bytes[numBytes]
		msgLen = msgLen | int(uint(byteRead&0x7F)<<numBitsToShift[numBytes])
		numBytes++
		moreBytes = (byteRead & 0x80) != 0
	}

	if msgLen+numBytes > len(bytes) {
		return nil, fmt.Errorf("Not enough data in message, message length = %d, length section bytes = %d, data length = %d", msgLen, numBytes, len(bytes))
	}

	return bytes[numBytes : numBytes+msgLen], nil
}

func (s *SignalrCoreCommon) ProcessJsonLatency(target string) {
	for msgReceived := range s.received {
		// Multiple json responses may be merged to be a list.
		// Split them and remove '0x1e' terminator.
		dataArray := bytes.Split(msgReceived.Content, []byte{0x1e})
		for _, msg := range dataArray {
			if len(msg) == 0 {
				// ignore the empty msg generated by the above bytes.Split
				continue
			}
			var common SignalRCommon
			err := json.Unmarshal(msg, &common)
			if err != nil {
				s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode incoming message common header", err)
				continue
			}

			// ignore ping
			if common.Type != 1 {
				continue
			}

			var content SignalRCoreInvocation
			err = json.Unmarshal(msg, &content)
			if err != nil {
				s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode incoming SignalR invocation message", err)
				continue
			}

			if content.Type == 1 && content.Target == target {
				msgPayload := content.Arguments[1]
				if pos := strings.Index(msgPayload, " "); pos >= 0 {
					msgPayload = msgPayload[0:pos]
				}
				sendStart, err := strconv.ParseInt(msgPayload, 10, 64)
				if err != nil {
					s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode start timestamp", err)
					continue
				}
				s.LogLatency((time.Now().UnixNano() - sendStart) / 1000000)
			}
		}
	}
}

func (s *SignalrCoreCommon) ProcessMsgPackLatency(target string) {
	for msgReceived := range s.received {
		msg, err := s.ParseBinaryMessage(msgReceived.Content)
		if err != nil {
			s.LogError("message:decode_error", msgReceived.ClientID, "Failed to parse incoming message", err)
			continue
		}
		var content MsgpackInvocation
		err = msgpack.Unmarshal(msg, &content)
		if err != nil {
			s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode incoming message", err)
			continue
		}

		if content.Target == target {
			sendStart, err := strconv.ParseInt(content.Params[1], 10, 64)
			if err != nil {
				s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode start timestamp", err)
				continue
			}
			s.LogLatency((time.Now().UnixNano() - sendStart) / 1000000)
		}
	}
}

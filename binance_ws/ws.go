package binance_ws

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"move_profit/binance_api"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type status int

const (
	disconnected status = iota
	connected
	reconnecting

	// listenKey 有效期为 60m，为确保消息不丢失至少 59m50s 刷新一下
	maxListenKeyRefreshInterval     = time.Minute*59 + time.Second*50
	defaultListenKeyRefreshInterval = "55m"

	// 重试最大次数：用于连接重连次数以及 api 请求次数
	defaultMaxRetryTimes = 100

	// 默认 msg chan 长度 2000
	DefaultMsgChanLength = 2000

	EventListenKeyExpired = "listenKeyExpired"
)

type WsService struct {
	logger   *logging.Logger
	conf     *ConnConf
	clientMu *sync.Mutex

	subscribeMsg []SubscribeMsgRequest

	publicClient         *websocket.Conn
	privacyClient        *websocket.Conn
	publicStatus         status
	privacyStatus        status
	publicMsgChan        chan []byte
	privacyMsgChan       chan []byte
	restartChan          chan int //ws断线重连
	privacyMu            *sync.RWMutex
	listenKey            string
	listenKeyRefreshTime time.Time
	expireChan           chan struct{}
}

type ConnConf struct {
	UserId                   uint32
	ApiUrl                   string
	URL                      string
	Key                      string
	Secret                   string
	IsOpenPublicWs           bool   // 是否开启 ws 公共频道订阅
	IsOpenPrivacyWs          bool   // 是否开启 ws 私有频道订阅
	PublicChanLen            uint64 // 公共通道消息存储长度
	PrivacyChanLen           uint64 // 私有通道消息存储长度
	ListenKeyRefreshInterval string // listenKey 刷新时间间隔
	MaxRetryConn             int
	SkipTlsVerify            bool
}

func NewWsService(logger *logging.Logger, conf *ConnConf) (*WsService, error) {
	if err := checkConf(conf); err != nil {
		return nil, err
	}

	ws := &WsService{
		logger:       logger,
		conf:         conf,
		clientMu:     new(sync.Mutex),
		subscribeMsg: make([]SubscribeMsgRequest, 0),
		expireChan:   make(chan struct{}, 10),
		privacyMu:    new(sync.RWMutex),
	}

	return ws, nil
}

func checkConf(conf *ConnConf) error {
	if !conf.IsOpenPublicWs && !conf.IsOpenPrivacyWs {
		return fmt.Errorf("both public ws and privacy ws were closed, must open one")
	}

	if conf.ListenKeyRefreshInterval != "" {
		du, err := time.ParseDuration(conf.ListenKeyRefreshInterval)
		if err != nil {
			return err
		}

		if du <= 0 {
			return fmt.Errorf("listenKey refresh interval must great than 0")
		}

		if du > maxListenKeyRefreshInterval {
			return fmt.Errorf("listenKey refresh interval must less than 60m")
		}
	} else {
		conf.ListenKeyRefreshInterval = defaultListenKeyRefreshInterval
	}

	if conf.MaxRetryConn <= 0 {
		conf.MaxRetryConn = defaultMaxRetryTimes
	}

	if conf.PublicChanLen == 0 {
		conf.PublicChanLen = DefaultMsgChanLength
	}

	if conf.PrivacyChanLen == 0 {
		conf.PrivacyChanLen = DefaultMsgChanLength
	}

	return nil
}

func (ws *WsService) WriteSubscribeMsg(req SubscribeMsgRequest) error {
	if !ws.conf.IsOpenPublicWs {
		return fmt.Errorf("public client not open, unable to operate")
	}

	byteReq, err := json.Marshal(req)
	if err != nil {
		ws.logger.Warningf("req Marshal err:%s", err.Error())
		return err
	}
	ws.clientMu.Lock()
	defer func() {
		ws.clientMu.Unlock()
	}()

	err = ws.publicClient.WriteMessage(websocket.TextMessage, byteReq)
	if err != nil {
		ws.logger.Warningf("public client write err:%s, req:%+v", err.Error(), req)
		return err
	}

	if !req.isReconnect {
		ws.subscribeMsg = append(ws.subscribeMsg, req)
	}

	return nil
}

func (ws *WsService) Start(initChan chan struct{}) {
	ws.logger.Warning("ws service started")

	err := ws.initClient()
	if err != nil {
		ws.logger.Error(err)
		return
	}

	select {
	case initChan <- struct{}{}:
	case <-time.After(time.Second * 10):
		ws.logger.Error("init ws failed, please check it!")
		return
	}

	if ws.conf.ListenKeyRefreshInterval != "" {
		refreshDu, err := time.ParseDuration(ws.conf.ListenKeyRefreshInterval)
		if err != nil {
			ws.logger.Errorf("failed to parse ListenKeyRefreshInterval[%s]:%s", ws.conf.ListenKeyRefreshInterval, err.Error())
			return
		}
		go func(ws *WsService, refreshDu time.Duration) {
			defer func() {
				if r := recover(); r != nil {
					ws.logger.Errorf("handler err:%+v", r)
					return
				}
			}()

			ws.refreshListenKey(refreshDu)
		}(ws, refreshDu)
	}

	//初始化的时候也得去重新获取一下仓位信息
	ws.restartChan <- 1
}

func (ws *WsService) setListenKey(key string) {
	ws.privacyMu.Lock()
	defer ws.privacyMu.Unlock()

	ws.listenKey = key
	ws.listenKeyRefreshTime = time.Now()
}

func (ws *WsService) getPrivacyUrl() string {
	ws.privacyMu.Lock()
	defer ws.privacyMu.Unlock()

	return fmt.Sprintf("%s/%s", ws.conf.URL, ws.listenKey)
}

func (ws *WsService) refreshListenKey(du time.Duration) {
	ticker := time.NewTicker(du)
	for {
		select {
		case <-ticker.C:
			isSuccess := false
			for i := 0; i < ws.conf.MaxRetryConn; i++ {
				err := binance_api.RefreshListenKey(ws.conf.ApiUrl, ws.conf.Key)
				if err != nil {
					ws.logger.Warningf("failed to refresh listkenKey:%s, retry %d times", err.Error(), i)
					time.Sleep(time.Millisecond * 100 * time.Duration(i))
				} else {
					isSuccess = true
					break
				}
			}

			if !isSuccess {
				ws.logger.Warningf("after %d times retry to refresh listkenKey then unable to get success, give it up now", ws.conf.MaxRetryConn)
			}
		case <-ws.expireChan:
			var listenKey string
			var err error

			isSuccess := false
			for i := 0; i < ws.conf.MaxRetryConn; i++ {
				// listenKey 过期，手动刷新重连; 即时 listenKey 过期，链接也不会主动断开
				listenKey, err = binance_api.GetListenKey(ws.conf.ApiUrl, ws.conf.Key, ws.conf.Secret)
				if err != nil {
					ws.logger.Warningf("failed to get listenKey:%s, unable to get a new listenKey for expire event, retry %d times", err.Error(), i)
					time.Sleep(time.Millisecond * 100 * time.Duration(i))
				} else {
					isSuccess = true
					break
				}
			}

			if !isSuccess {
				ws.logger.Warningf("after %d times retry to get listkenKey for expire event then unable to get success, give it up now", ws.conf.MaxRetryConn)
				continue
			}

			ws.logger.Warningf("privacy client will reconnect with new listenKey %s for expire event", listenKey)
			ws.setListenKey(listenKey)

			err = ws.reconnectPrivacy()
			if err != nil {
				ws.logger.Warningf("failed to reconnect privacy client for expire event:%s", err.Error())
			}
		}
	}
}

func (ws *WsService) GetPublicMsgChan() (<-chan []byte, error) {
	if !ws.conf.IsOpenPublicWs {
		return nil, fmt.Errorf("public ws opened")
	}

	return ws.publicMsgChan, nil
}

func (ws *WsService) GetPrivacyMsgChan() (<-chan []byte, error) {
	if !ws.conf.IsOpenPrivacyWs {
		return nil, fmt.Errorf("privacy ws opened")
	}

	return ws.privacyMsgChan, nil
}

func (ws *WsService) GetRestartMsgChan() (chan int, error) {
	return ws.restartChan, nil
}

func (ws *WsService) initClient() error {
	if ws.conf.IsOpenPublicWs {
		ws.logger.Warning("public client init start")

		msgChan := make(chan []byte, ws.conf.PublicChanLen)
		ws.publicMsgChan = msgChan

		conn, err := ws.initConn(ws.conf.URL)
		if err != nil {
			return err
		}

		ws.clientMu.Lock()
		ws.publicStatus = connected
		ws.clientMu.Unlock()

		ws.publicClient = conn
		go func(ws *WsService) {
			defer func() {
				if r := recover(); r != nil {
					ws.logger.Errorf("handler err:%+v", r)
					return
				}
			}()

			ws.readPublicMsg()
		}(ws)
		ws.logger.Warning("public client init done")
	}
	ws.restartChan = make(chan int, 100)
	if ws.conf.IsOpenPrivacyWs {
		ws.logger.Warning("privacy client init")

		msgChan := make(chan []byte, ws.conf.PrivacyChanLen)
		ws.privacyMsgChan = msgChan

		listenKey, err := binance_api.GetListenKey(ws.conf.ApiUrl, ws.conf.Key, ws.conf.Secret)
		if err != nil {
			return fmt.Errorf("failed to get listenKey:%s, unable to start privacy ws", err.Error())
		}
		ws.logger.Warningf("privacy client start with listenKey %s", listenKey)
		ws.setListenKey(listenKey)

		conn, err := ws.initConn(ws.getPrivacyUrl())
		if err != nil {
			return err
		}

		ws.clientMu.Lock()
		ws.privacyStatus = connected
		ws.clientMu.Unlock()

		ws.privacyClient = conn

		go func(ws *WsService) {
			defer func() {
				if r := recover(); r != nil {
					ws.logger.Errorf("handler err:%+v", r)
					return
				}
			}()

			ws.readPrivacyMsg()
		}(ws)
		ws.logger.Warningf("privacy client init done user_id:%d", ws.conf.UserId)
	}

	return nil
}

func (ws *WsService) initConn(urlStr string) (*websocket.Conn, error) {
	stop := false
	retry := 0
	var conn *websocket.Conn
	for !stop {
		dialer := websocket.DefaultDialer
		if ws.conf.SkipTlsVerify {
			dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		c, resp, err := dialer.Dial(urlStr, nil)
		if err != nil {
			if retry >= ws.conf.MaxRetryConn {
				ws.logger.Warningf("max reconnect time %d reached, give it up", ws.conf.MaxRetryConn)
				return nil, err
			}
			retry++
			ws.logger.Warningf("failed to connect to server for the %d time, try again later. err: %v, resp: %v", retry, err, resp)
			time.Sleep(time.Millisecond * (time.Duration(retry) * 100))
			continue
		} else {
			stop = true
			conn = c
		}
	}

	if retry > 0 {
		ws.logger.Warningf("reconnect succeeded after retrying %d times", retry)
	}

	return conn, nil
}

type SubscribeMsgRequest struct {
	Id          int64         `json:"id"`
	Method      string        `json:"method"`
	Params      []interface{} `json:"params"`
	isReconnect bool
}

type ResponseMsg struct {
	e string `json:"e"` // event
}

func (ws *WsService) readPublicMsg() {
	defer ws.publicClient.Close()

	for {
		select {
		default:
			_, message, err := ws.publicClient.ReadMessage()
			if err != nil {
				ws.logger.Warningf("public client websocket err: %s", err.Error())
				if e := ws.reconnectPublic(); e != nil {
					ws.logger.Warningf("reconnect public client err:%s", err.Error())
					return
				} else {
					ws.logger.Info("reconnect public client success, continue read message")
					continue
				}
			}

			select {
			case ws.publicMsgChan <- message:

			}
		}
	}
}

func (ws *WsService) readPrivacyMsg() {
	defer ws.privacyClient.Close()

	for {
		select {
		default:
			_, message, err := ws.privacyClient.ReadMessage()
			if err != nil {
				ws.logger.Warningf("privacy client websocket err: %s", err.Error())
				if e := ws.reconnectPrivacy(); e != nil {
					ws.logger.Warningf("reconnect privacy client err:%s", err.Error())
					return
				} else {
					ws.logger.Info("reconnect privacy client success, continue read message")
					continue
				}
			}

			var resp ResponseMsg
			err = json.Unmarshal(message, &resp)
			if err != nil {
				ws.logger.Warningf("failed to Unmarshal message [%s]:%s", string(message), err.Error())
			} else {
				if resp.e == EventListenKeyExpired {
					// 发送过期 listenKey 消息，去进行主动重连
					ws.expireChan <- struct{}{}
				}
			}

			select {
			case ws.privacyMsgChan <- message:

			}
		}
	}
}

func (ws *WsService) reconnectPublic() error {
	// avoid repeated reconnection
	if ws.publicStatus == reconnecting {
		return nil
	}

	ws.clientMu.Lock()
	defer ws.clientMu.Unlock()

	if ws.publicClient != nil {
		ws.publicClient.Close()
	}

	ws.publicStatus = reconnecting
	ws.restartChan <- 1

	stop := false
	retry := 0
	for !stop {
		c, _, err := websocket.DefaultDialer.Dial(ws.conf.URL, nil)
		if err != nil {
			if retry >= ws.conf.MaxRetryConn {
				ws.logger.Warningf("max reconnect time %d reached, give it up", ws.conf.MaxRetryConn)
				return err
			}
			retry++
			ws.logger.Warningf("failed to connect to server for the %d times, try again later", retry)
			time.Sleep(time.Millisecond * (time.Duration(retry) * 500))
			continue
		} else {
			stop = true
			ws.publicClient = c
		}
	}

	ws.publicStatus = connected

	// resubscribe after reconnect
	for _, req := range ws.subscribeMsg {
		req.isReconnect = true
		err := ws.WriteSubscribeMsg(req)
		if err != nil {
			ws.logger.Warningf("failed to subscribe at reconnect, req:%+v, err:%s", req, err)
		}
	}

	return nil
}

func (ws *WsService) reconnectPrivacy() error {
	// avoid repeated reconnection
	if ws.privacyStatus == reconnecting {
		return nil
	}

	ws.clientMu.Lock()
	defer ws.clientMu.Unlock()

	if ws.privacyClient != nil {
		ws.privacyClient.Close()
	}

	ws.privacyStatus = reconnecting
	ws.restartChan <- 1

	stop := false
	retry := 0
	for !stop {
		c, _, err := websocket.DefaultDialer.Dial(ws.getPrivacyUrl(), nil)
		if err != nil {

			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ws.logger.Warningf("failed to dial %s:%s, next will retry", ws.getPrivacyUrl(), err.Error())
			} else {
				// timeout
				ws.logger.Warningf("failed to dial %s timeout:%s, manual get a new listenKey to retry", ws.getPrivacyUrl(), err.Error())

				listenKey, err := binance_api.GetListenKey(ws.conf.ApiUrl, ws.conf.Key, ws.conf.Secret)
				if err != nil {
					return fmt.Errorf("failed to get listenKey:%s, unable to get a new listenKey for reconnect", err.Error())
				}
				ws.logger.Warningf("privacy client will reconnect with new listenKey %s", listenKey)
				ws.setListenKey(listenKey)
			}

			if retry >= ws.conf.MaxRetryConn {
				ws.logger.Warningf("max reconnect time %d reached, give it up", ws.conf.MaxRetryConn)
				return err
			}

			retry++
			ws.logger.Warningf("failed to connect to server for the %d time, try again later", retry)
			time.Sleep(time.Millisecond * (time.Duration(retry) * 500))
			continue
		} else {
			stop = true
			ws.privacyClient = c
		}
	}

	ws.privacyStatus = connected

	return nil
}

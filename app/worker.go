package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"encoding/base64"
)

var uptime = time.Now().Unix()

type Amount struct {
	value int64
}

func (a Amount) Value() int64 {
	return a.value
}

func (a *Amount) UnmarshalJSON(b []byte) error {
	str := string(b)
	s := 0
	e := len(b)
	if len(str) > 2 && str[0] == '"' && str[len(b)-1] == '"' {
		s = 1
		e = len(b) - 1
	}
	var err error
	a.value, err = strconv.ParseInt(str[s:e], 10, 64)
	return err
}

type ServerResponse struct {
	SubscriptionKey string `json:"subscriptionKey,omitempty"`

	Params struct {
		Model struct {
			Status string `json:"status,omitempty"`
		} `json:"model,omitempty"`
		User struct {
			Status string `json:"status,omitempty"`
		} `json:"user,omitempty"`
		ClientId string `json:"clientId,omitempty"`
		Message  struct {
			Type     string `json:"type,omitempty"`
			Userdata struct {
				Username string `json:"username,omitempty"`
			} `json:"userdata,omitempty"`
			Details struct {
				Amount         Amount `json:"amount,omitempty"`
				LovenseDetails struct {
					Type   string `json:"type,omitempty"`
					Detail struct {
						Name   string `json:"name,omitempty"`
						Amount Amount `json:"amount,omitempty"`
					} `json:"detail,omitempty"`
				} `json:"lovenseDetails"`
			} `json:"details,omitempty"`
		} `json:"message,omitempty"`
	} `json:"params,omitempty"`
}

func mapRooms() {

	data := make(map[string]*Info)

	for {
		select {
		case m := <-rooms.Add:
			data[m.room] = &Info{Id: m.Id, Rid: m.Rid, Server: m.Server, Proxy: m.Proxy, Start: m.Start, Last: m.Last, Online: m.Online, Income: m.Income, Dons: m.Dons, Tips: m.Tips, ch: m.ch}

		case s := <-rooms.Json:
			j, err := json.Marshal(data)
			if err == nil {
				s = string(j)
			}
			rooms.Json <- s

		case <-rooms.Count:
			rooms.Count <- len(data)

		case key := <-rooms.Del:
			delete(data, key)

		case room := <-rooms.Check:
			if _, ok := data[room]; !ok {
				room = ""
			}
			rooms.Check <- room

		case room := <-rooms.Stop:
			if _, ok := data[room]; ok {
				close(data[room].ch)
			}
		}
	}
}

func announceCount() {
	for {
		time.Sleep(30 * time.Second)
		rooms.Count <- 0
		l := <-rooms.Count
		msg, err := json.Marshal(struct {
			Chanel string `json:"chanel"`
			Count  int    `json:"count"`
		}{
			Chanel: "stripchat",
			Count:  l,
		})
		if err == nil {
			socketServer <- msg
		}
	}
}

func reconnectRoom(workerData Info) {
	time.Sleep(5 * time.Second)
	fmt.Println("reconnect:", workerData.room, workerData.Id, workerData.Proxy)
	startRoom(workerData)
}

func xWorker(workerData Info) {
	fmt.Println("Start", workerData.room, "id", workerData.Id, "proxy", workerData.Proxy)

	rooms.Add <- workerData

	defer func() {
		rooms.Del <- workerData.room
	}()


	if len(workerData.Server) < 50 {
		fmt.Println(workerData.Server, workerData.room)
		return
	}
	
	wsUrl, err := base64.StdEncoding.DecodeString(workerData.Server)
	if err != nil {
		fmt.Println(err, workerData.room)
		return
	}

	u, err := url.Parse(string(wsUrl))
	if err != nil {
		fmt.Println(err, workerData.room)
		return
	}

	Dialer := *websocket.DefaultDialer

	if _, ok := conf.Proxy[workerData.Proxy]; ok {
		Dialer = websocket.Dialer{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http", // or "https" depending on your proxy
				Host:   conf.Proxy[workerData.Proxy],
				Path:   "/",
			}),
			HandshakeTimeout: 45 * time.Second, // https://pkg.go.dev/github.com/gorilla/websocket
		}
	}

	c, _, err := Dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err.Error(), u.String(), workerData.room)
		return
	}

	defer c.Close()

	dons := make(map[string]struct{})
	
	ticker := time.NewTicker(60 * 60 * 8 * time.Second)
	defer ticker.Stop()
	
	var income int64
	income = 0

	for {
		
		select {
		case <-ticker.C:
			fmt.Println("too_long exit:", workerData.room)
			return
		case <-workerData.ch:
			fmt.Println("Exit room:", workerData.room)
			return
		default:
		}
		
		c.SetReadDeadline(time.Now().Add(30 * time.Minute))
		_, message, err := c.ReadMessage()
		if err != nil {
			fmt.Println(err.Error(), workerData.room)
			if income > 1 && websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				go reconnectRoom(workerData)
			}
			return
		}

		now := time.Now().Unix()
		slog <- saveLog{workerData.Rid, now, string(message)}
		
		if now > workerData.Last+60*60 {
			fmt.Println("no_tips exit:", workerData.room)
			return
		}
		
		m := &ServerResponse{}
		if err = json.Unmarshal(message, m); err != nil {
			fmt.Println(err.Error(), workerData.room)
			continue
		}

		if m.SubscriptionKey == "connected" {
			id := workerData.Id
			messages := [][]byte{}
			mTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-lotteryChanged","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/lotteryChanged"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-userBanned:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/userBanned:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-goalChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/goalChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-modelStatusChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/modelStatusChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-broadcastSettingsChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/broadcastSettingsChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-tipMenuUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/tipMenuUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-topicChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/topicChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-userUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/userUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-interactiveToyStatusChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/interactiveToyStatusChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-groupShow:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/groupShow:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-deleteChatMessages:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/deleteChatMessages:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-tipLeaderboardSettingsUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/tipLeaderboardSettingsUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-modelAppUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/modelAppUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-newKing:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/newKing:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-privateMessageSettingsChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/privateMessageSettingsChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-newChatMessage:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/newChatMessage:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-fanClubUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/fanClubUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"`+mTime+`-sub-viewServerChanged:hls-07","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/viewServerChanged:hls-07"}`))
			for _, msg := range messages {
				if err = c.WriteMessage(websocket.TextMessage, msg); err != nil {
					fmt.Println(err.Error(), workerData.room)
					return
				}
			}
			messages = nil
			continue
		}

		if strings.Contains(m.SubscriptionKey, "userUpdated") && m.Params.User.Status == "off" {
			fmt.Println("userUpdated exiting", workerData.room)
			return
		}

		if strings.Contains(m.SubscriptionKey, "modelStatusChanged") && m.Params.Model.Status == "off" {
			fmt.Println("modelStatusChanged exiting", workerData.room)
			return
		}

		if m.Params.Message.Type == "tip" {
			if len(m.Params.Message.Userdata.Username) < 3 {
				m.Params.Message.Userdata.Username = "anon_tips"
			}

			if _, ok := dons[m.Params.Message.Userdata.Username]; !ok {
				dons[m.Params.Message.Userdata.Username] = struct{}{}
				workerData.Dons++
			}
			
			save <- saveData{workerData.room, strings.ToLower(m.Params.Message.Userdata.Username), workerData.Rid, m.Params.Message.Details.Amount.Value(), now}

			income += m.Params.Message.Details.Amount.Value()
			
			workerData.Tips++
			workerData.Last = now
			workerData.Income += m.Params.Message.Details.Amount.Value()
			rooms.Add <- workerData
			
			fmt.Println(m.Params.Message.Userdata.Username, "send", m.Params.Message.Details.Amount.Value(), "tokens to", workerData.room)
		}
	}
}

package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
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
			Count int `json:"count"`
		}{Count: l})
		if err == nil {
			ws.Send <- msg
		}
	}
}

func getToken(room string) string {
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, "https://stripchat.com/"+room, nil)
	if err != nil {
		return "cant get req"
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:33.0) Gecko/20100101 Firefox/33.0")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Add("Accept-Language", "en-US,en;q=0.5")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Referer", "https://stripchat.com")
	rsp, err := http.DefaultClient.Do(req)
	if err != nil || rsp.StatusCode != http.StatusOK {
		return "cant get page"
	}
	defer rsp.Body.Close()
	buf, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return "cant read page"
	}
	re := regexp.MustCompile(`"websocketUrl":"*(.*?)\s*"`)
	m := re.FindSubmatch(buf)
	if len(m) != 2 {
		return "cant get ws"
	}
	return string(bytes.ReplaceAll(m[1], []byte(`\u002F`), []byte(`/`)))
}

func reconnectRoom(workerData Info) {
	n := randInt(10, 30)
	fmt.Printf("Sleeping %d seconds...\n", n)
	time.Sleep(time.Duration(n) * time.Second)
	fmt.Println("reconnect:", workerData.room, workerData.Id, workerData.Proxy)
	workerData.Last = time.Now().Unix()
	startRoom(workerData)
}

func xWorker(workerData Info) {
	fmt.Println("Start", workerData.room, "id", workerData.Id, "proxy", workerData.Proxy)

	rooms.Add <- workerData

	defer func() {
		rooms.Del <- workerData.room
	}()

	if workerData.Server == "" {
		workerData.Server = getToken(workerData.room)
	}
	
	if len(workerData.Server) < 50 {
		fmt.Println(workerData.Server, workerData.room)
		return
	}

	u, err := url.Parse(workerData.Server)
	if err != nil {
		fmt.Println(err, workerData.room)
		return
	}

	Dialer := *websocket.DefaultDialer

	proxyMap := make(map[string]string)
	proxyMap["us"] = "5.161.128.20:3128"
	proxyMap["fi"] = "65.21.180.188:3128"

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

	for {
		c.SetReadDeadline(time.Now().Add(30 * time.Minute))
		_, message, err := c.ReadMessage()
		if err != nil {
			fmt.Println(err.Error(), workerData.room)
			if workerData.Income > 1 && websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				reconnectRoom(workerData)
			}
			return
		}

		now := time.Now().Unix()

		slog <- saveLog{workerData.Rid, time.Now().Unix(), string(message)}

		m := &ServerResponse{}

		if err = json.Unmarshal(message, m); err != nil {
			fmt.Println(err.Error(), workerData.room)
			continue
		}

		workerData.Last = now
		rooms.Add <- workerData

		if m.SubscriptionKey == "connected" {
			id := workerData.Id
			messages := [][]byte{}
			messages = append(messages, []byte(`{"id":"1660248194970-sub-lotteryChanged","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/lotteryChanged"}`))
			messages = append(messages, []byte(`{"id":"1660248194970-sub-userBanned:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/userBanned:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194970-sub-goalChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/goalChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194970-sub-modelStatusChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/modelStatusChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-broadcastSettingsChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/broadcastSettingsChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-tipMenuUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/tipMenuUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-topicChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/topicChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-userUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/userUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-interactiveToyStatusChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/interactiveToyStatusChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-groupShow:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/groupShow:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-deleteChatMessages:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/deleteChatMessages:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-tipLeaderboardSettingsUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/tipLeaderboardSettingsUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194971-sub-modelAppUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/modelAppUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194972-sub-newKing:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/newKing:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194972-sub-privateMessageSettingsChanged:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/privateMessageSettingsChanged:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194972-sub-newChatMessage:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/newChatMessage:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194972-sub-fanClubUpdated:`+id+`","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/fanClubUpdated:`+id+`"}`))
			messages = append(messages, []byte(`{"id":"1660248194972-sub-viewServerChanged:hls-07","method":"PUT","url":"/front/clients/`+m.Params.ClientId+`/subscriptions/viewServerChanged:hls-07"}`))
			for _, msg := range messages {
				if err = c.WriteMessage(websocket.TextMessage, msg); err != nil {
					fmt.Println(err.Error(), workerData.room)
					return
				}
			}
			messages = nil
		}

		if strings.Contains(m.SubscriptionKey, "userUpdated") && m.Params.User.Status == "off" {
			fmt.Println("user exiting", workerData.room)
			return
		}

		if strings.Contains(m.SubscriptionKey, "modelStatusChanged") && m.Params.Model.Status == "off" {
			fmt.Println("user exiting", workerData.room)
			return
		}

		if m.Params.Message.Type == "tip" {

			if len(m.Params.Message.Userdata.Username) < 3 {
				continue
			}

			fmt.Println(m.Params.Message.Userdata.Username, "send", m.Params.Message.Details.Amount.Value(), "tokens")

			workerData.Tips++
			if _, ok := dons[m.Params.Message.Userdata.Username]; !ok {
				dons[m.Params.Message.Userdata.Username] = struct{}{}
				workerData.Dons++
			}

			save <- saveData{workerData.room, strings.ToLower(m.Params.Message.Userdata.Username), workerData.Rid, m.Params.Message.Details.Amount.Value(), now}

			workerData.Income += m.Params.Message.Details.Amount.Value()
			rooms.Add <- workerData
		}
	}
}

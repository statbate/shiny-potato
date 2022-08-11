package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var uptime = time.Now().Unix()

type AuthResponse struct {
	Status    string `json:"status"`
	LocalData struct {
		DataKey string `json:"dataKey"`
	} `json:"localData"`
	UserData struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Location    string `json:"location"`
		Chathost    string `json:"chathost"`
		IsRu        bool   `json:"isRu"`
	} `json:"userData"`
}

type ServerResponse struct {
	TS   int64               `json:"ts"`
	Type string              `json:"type"`
	Body jsoniter.RawMessage `json:"body"`
}

type DonateResponse struct {
	F struct {
		Username string `json:"username"`
	} `json:"f"`
	A int64 `json:"a"`
}

func mapRooms() {

	data := make(map[string]*Info)

	for {
		select {
		case m := <-rooms.Add:
			data[m.room] = &Info{Server: m.Server, Proxy: m.Proxy, Start: m.Start, Last: m.Last, Online: m.Online, Income: m.Income, Dons: m.Dons, Tips: m.Tips, ch: m.ch}

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

func getAMF(room string) (bool, *AuthResponse) {

	v := &AuthResponse{}

	req, err := http.NewRequest(http.MethodPost, "https://rt.bongocams.com/tools/amf.php?res=771840&t=1654437233142", strings.NewReader(`method=getRoomData&args[]=`+room))
	if err != nil {
		fmt.Println(err.Error())
		return false, v
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Add("X-Requested-With", "XMLHttpRequest")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Referrer", "https://bongacams.com")
	req.Header.Add("User-agent", "curl/7.79.1")

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return false, v
	}
	defer rsp.Body.Close()

	if err = json.NewDecoder(rsp.Body).Decode(v); err != nil {
		fmt.Println(err.Error())
		return false, v
	}

	return true, v
}

func xWorker(workerData Info, u url.URL) {
	fmt.Println("Start", workerData.room, "server", workerData.Server, "proxy", workerData.Proxy)

	rooms.Add <- workerData

	defer func() {
		rooms.Del <- workerData.room
	}()

	ok, v := getAMF(workerData.room)
	if !ok {
		fmt.Println("exit: no amf parms")
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
		fmt.Println(err.Error(), workerData.room)
		return
	}

	defer c.Close()

	c.SetReadDeadline(time.Now().Add(60 * time.Second))

	if err = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":%d,"name":"joinRoom","args":["%s",{"username":"%s","displayName":"%s","location":"%s","chathost":"%s","isRu":%t,"isPerformer":false,"hasStream":false,"isLogged":false,"isPayable":false,"showType":"public"},"%s"]}`, 1, v.UserData.Chathost, v.UserData.Username, v.UserData.DisplayName, v.UserData.Location, v.UserData.Chathost, v.UserData.IsRu, v.LocalData.DataKey))); err != nil {
		fmt.Println(err.Error())
		return
	}

	_, message, err := c.ReadMessage()
	if err != nil {
		fmt.Println(err.Error(), workerData.room)
		return
	}

	slog <- saveLog{workerData.Rid, time.Now().Unix(), string(message)}

	if string(message) == `{"id":1,"result":{"audioAvailable":false,"freeShow":false},"error":null}` {
		fmt.Println("room offline, exit", workerData.room)
		return
	}

	if err = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":%d,"name":"ChatModule.connect","args":["public-chat"]}`, 2))); err != nil {
		fmt.Println(err.Error(), workerData.room)
		return
	}

	_, message, err = c.ReadMessage()
	if err != nil {
		fmt.Println(err.Error(), workerData.room)
		return
	}

	slog <- saveLog{workerData.Rid, time.Now().Unix(), string(message)}

	quit := make(chan struct{})
	pid := 3

	defer func() {
		close(quit)
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				if err = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":%d,"name":"ping"}`, pid))); err != nil {
					fmt.Println(err.Error(), workerData.room)
					rooms.Stop <- workerData.room
					return
				}
				pid++
				break
			}
		}
	}()

	dons := make(map[string]struct{})

	for {
		select {
		case <-workerData.ch:
			fmt.Println("Exit room:", workerData.room)
			return
		default:
		}

		c.SetReadDeadline(time.Now().Add(30 * time.Minute))
		_, message, err := c.ReadMessage()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		now := time.Now().Unix()

		slog <- saveLog{workerData.Rid, now, string(message)}

		m := &ServerResponse{}

		if err = json.Unmarshal(message, m); err != nil {
			fmt.Println(err.Error(), workerData.room)
			continue
		}

		workerData.Last = now
		rooms.Add <- workerData

		if m.Type == "ServerMessageEvent:PERFORMER_STATUS_CHANGE" && string(m.Body) == `"offline"` {
			fmt.Println(m.Type, workerData.room)
			return
		}

		if m.Type == "ServerMessageEvent:ROOM_CLOSE" {
			fmt.Println(m.Type, workerData.room)
			return
		}

		if m.Type == "ServerMessageEvent:INCOMING_TIP" {
			d := &DonateResponse{}
			if err = json.Unmarshal(m.Body, d); err == nil {
				//fmt.Println(d.F.Username, "send", d.A, "tokens")

				workerData.Tips++
				if _, ok := dons[d.F.Username]; !ok {
					dons[d.F.Username] = struct{}{}
					workerData.Dons++
				}

				save <- saveData{workerData.room, d.F.Username, workerData.Rid, d.A, now}

				workerData.Income += d.A
				rooms.Add <- workerData
			}
		}
	}
}

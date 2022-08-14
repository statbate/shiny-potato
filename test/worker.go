package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Amount struct {
	value int
}

func (a Amount) Value() int {
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
	a.value, err = strconv.Atoi(str[s:e])
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

func startRoom(room, server, proxy string, u *url.URL, id string) {
	
	fmt.Println("Start", room, "server", server, "proxy", proxy)

	Dialer := *websocket.DefaultDialer

	proxyMap := make(map[string]string)
	proxyMap["us"] = "aaa:port"
	proxyMap["fi"] = "bbb:port"

	if _, ok := proxyMap[proxy]; ok {
		Dialer = websocket.Dialer{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http", // or "https" depending on your proxy
				Host:   proxyMap[proxy],
				Path:   "/",
			}),
			HandshakeTimeout: 45 * time.Second, // https://pkg.go.dev/github.com/gorilla/websocket
		}
	}

	c, _, err := Dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer c.Close()
	
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		//fmt.Println(string(message))

		m := &ServerResponse{}

		if err = json.Unmarshal(message, m); err != nil {
			fmt.Println(err.Error())
			continue
		}

		if m.SubscriptionKey == "connected" {
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
					fmt.Println(err.Error())
					return
				}
			}
			messages = nil
		}

		if strings.Contains(m.SubscriptionKey, "userUpdated") && m.Params.User.Status == "off" {
			fmt.Println("user exiting")
			return
		} 
		
		if strings.Contains(m.SubscriptionKey, "modelStatusChanged") && m.Params.Model.Status == "off" {
			fmt.Println("user exiting")
			return
		}

		if m.Params.Message.Type == "tip" {
			fmt.Println(m.Params.Message.Userdata.Username, " send ", m.Params.Message.Details.Amount.Value(), "tokens")
		}
	}
}

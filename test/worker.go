package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var messages [][]byte

func init() {
	messages = append(messages, []byte(`{"id":"1660248194970-sub-lotteryChanged","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/lotteryChanged"}`))
	messages = append(messages, []byte(`{"id":"1660248194970-sub-userBanned:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/userBanned:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194970-sub-goalChanged:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/goalChanged:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194970-sub-modelStatusChanged:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/modelStatusChanged:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-broadcastSettingsChanged:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/broadcastSettingsChanged:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-tipMenuUpdated:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/tipMenuUpdated:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-topicChanged:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/topicChanged:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-userUpdated:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/userUpdated:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-interactiveToyStatusChanged:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/interactiveToyStatusChanged:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-groupShow:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/groupShow:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-deleteChatMessages:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/deleteChatMessages:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-tipLeaderboardSettingsUpdated:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/tipLeaderboardSettingsUpdated:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194971-sub-modelAppUpdated:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/modelAppUpdated:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194972-sub-newKing:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/newKing:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194972-sub-privateMessageSettingsChanged:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/privateMessageSettingsChanged:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194972-sub-newChatMessage:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/newChatMessage:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194972-sub-fanClubUpdated:XID","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/fanClubUpdated:XID"}`))
	messages = append(messages, []byte(`{"id":"1660248194972-sub-viewServerChanged:hls-07","method":"PUT","url":"/front/clients/CLIENT_ID/subscriptions/viewServerChanged:hls-07"}`))
}

type ServerResponse struct {
	SubscriptionKey string `json:"subscriptionKey,omitempty"`
	Params          struct {
		ClientId string `json:"clientId,omitempty"`
		Message  struct {
			Type     string `json:"type,omitempty"`
			Userdata struct {
				Username string `json:"username,omitempty"`
			} `json:"userdata,omitempty"`
			Details struct {
				Amount         float64 `json:"amount,omitempty"`
				LovenseDetails struct {
					Type   string `json:"type,omitempty"`
					Detail struct {
						Name   string  `json:"name,omitempty"`
						Amount float64 `json:"amount,omitempty"`
					} `json:"detail,omitempty"`
				} `json:"lovenseDetails"`
			} `json:"details,omitempty"`
		} `json:"message,omitempty"`
	} `json:"params,omitempty"`
}

func startRoom(room, server, proxy string, u *url.URL, id string) {
	// curl -vvv -X POST -H "Content-Type: application/x-www-form-urlencoded; charset=UTF-8" -H "X-Requested-With: XMLHttpRequest" -d "method=getRoomData" -d "args[]=Icehotangel"   "https://rt.bongocams.com/tools/amf.php?res=771840&t=1654437233142"

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
			// fmt.Println("return " + err.Error())
			return
		}

		fmt.Println(string(message))

		m := &ServerResponse{}

		if err = json.Unmarshal(message, m); err != nil {
			fmt.Println(err.Error())
			continue
		}

		if m.SubscriptionKey == "connected" {
			fmt.Println("subscribe to messages")
			for _, msg := range messages {
				b := bytes.ReplaceAll(msg, []byte(`CLIENT_ID`), []byte(m.Params.ClientId))
				b = bytes.ReplaceAll(b, []byte(`XID`), []byte(id))
				if err = c.WriteMessage(websocket.TextMessage, b); err != nil {
					// fmt.Println("return " + err.Error())
					return
				}
			}

		}

		if !strings.Contains(m.SubscriptionKey, "newChatMessage") {
			continue
		}

		if m.Params.Message.Type == "lovense" {
			// fmt.Println("donate")
			fmt.Println(m.Params.Message.Details.LovenseDetails.Detail.Name, " send ", m.Params.Message.Details.LovenseDetails.Detail.Amount, "tokens")
		} else if m.Params.Message.Type == "tip" {
			// fmt.Println("donate")
			fmt.Println(m.Params.Message.Userdata.Username, " send ", m.Params.Message.Details.Amount, "tokens")
		}
	}
}

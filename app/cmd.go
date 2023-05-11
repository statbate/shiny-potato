package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

var memInfo runtime.MemStats

type Info struct {
	ch     chan struct{}
	room   string
	Id     string `json:"id"`
	Server string `json:"server"`
	Proxy  string `json:"proxy"`
	Online string `json:"online"`
	Rid    int64  `json:"rid"`
	Start  int64  `json:"start"`
	Last   int64  `json:"last"`
	Income int64  `json:"income"`
	Dons   int64  `json:"dons"`
	Tips   int64  `json:"tips"`
}

func updateFileRooms() string {
	for {
		rooms.Json <- ""
		s := <-rooms.Json
		err := os.WriteFile(conf.Conn["start"], []byte(s), 0644)
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(10 * time.Second)
	}
}

func listHandler(w http.ResponseWriter, _ *http.Request) {
	dat, err := os.ReadFile(conf.Conn["start"])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Fprint(w, string(dat))
}

func debugHandler(w http.ResponseWriter, _ *http.Request) {
	runtime.ReadMemStats(&memInfo)
	j, err := json.Marshal(struct {
		Goroutines int
		Uptime     int64
		Alloc      uint64
		HeapSys    uint64
	}{
		Goroutines: runtime.NumGoroutine(),
		Alloc:      memInfo.Alloc,
		HeapSys:    memInfo.HeapSys,
		Uptime:     uptime,
	})
	if err == nil {
		fmt.Fprint(w, string(j))
	}
}

func cmdHandler(w http.ResponseWriter, r *http.Request) {
	if !conf.List[r.Header.Get("X-REAL-IP")] {
		fmt.Fprint(w, "403")
		return
	}
	params := r.URL.Query()
	if len(params["room"]) > 0 && len(params["id"]) > 0 && len(params["proxy"]) > 0 {
		now := time.Now().Unix()
		workerData := Info{
			room:   params["room"][0],
			Id:     params["id"][0],
			Server: "",
			Proxy:  params["proxy"][0],
			Online: "0",
			Start:  now,
			Last:   now,
			Rid:    0,
			Income: 0,
			Dons:   0,
			Tips:   0,
		}
		startRoom(workerData)
	}
	if len(params["exit"]) > 0 {
		rooms.Stop <- strings.Join(params["exit"], "")
	}
	fmt.Fprint(w, string("ok"))
}

func startRoom(workerData Info) {
	rooms.Check <- workerData.room
	testRoom := <-rooms.Check
	if testRoom == workerData.room {
		fmt.Println("Already track:", workerData.room)
		return
	}

	rid, ok := getRoomInfo(workerData.room)
	if !ok {
		fmt.Println("No room in MySQL:", workerData.room)
		return
	}

	workerData.Rid = rid
	workerData.ch = make(chan struct{})

	go xWorker(workerData)
}

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	jsoniter "github.com/json-iterator/go"
)

type Rooms struct {
	Count chan int
	Json  chan string
	Check chan string
	Stop  chan string
	Del   chan string
	Add   chan Info
}

var (
	Mysql, Clickhouse *sqlx.DB

	json = jsoniter.ConfigCompatibleWithStandardLibrary

	socketServer = make(chan []byte, 1)

	save = make(chan saveData, 1)
	slog = make(chan saveLog, 32)

	rooms = &Rooms{
		Count: make(chan int),
		Json:  make(chan string),
		Check: make(chan string),
		Stop:  make(chan string),
		Del:   make(chan string),
		Add:   make(chan Info),
	}
)

func main() {
	startConfig()

	initMysql()
	initClickhouse()

	go mapRooms()
	go announceCount()
	go saveDB()
	go saveLogs()
	go socketHandler()

	http.HandleFunc("/stripchat/cmd/", cmdHandler)
	http.HandleFunc("/stripchat/list/", listHandler)
	http.HandleFunc("/stripchat/debug/", debugHandler)

	go fastStart()

	const SOCK = "/tmp/stripchat.sock"
	os.Remove(SOCK)
	unixListener, err := net.Listen("unix", SOCK)
	if err != nil {
		log.Fatal("Listen (UNIX socket): ", err)
	}
	defer unixListener.Close()
	os.Chmod(SOCK, 0777)
	log.Fatal(http.Serve(unixListener, nil))
}

func initMysql() {
	db, err := sqlx.Connect("mysql", conf.Conn["mysql"])
	if err != nil {
		panic(err)
	}
	Mysql = db
}

func initClickhouse() {
	db, err := sqlx.Connect("clickhouse", conf.Conn["click"])
	if err != nil {
		panic(err)
	}
	Clickhouse = db
}

func socketHandler() {

	var (
		err  error
		conn net.Conn
	)

	for {
		select {
		case b := <-socketServer:

			if conn == nil || conn.RemoteAddr() == nil {
				conn, err = net.DialTimeout("unix", "/tmp/echo.sock", time.Millisecond * 10)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
			}

			if conn != nil {
				if _, err = conn.Write(b); err != nil {
					fmt.Println(err.Error())
					conn.Close()
					conn = nil
				}
			}

		}
	}
}

func fastStart() {
	defer func() {
		go updateFileRooms()
	}()
	dat, err := os.ReadFile(conf.Conn["start"])
	if err != nil {
		fmt.Println(err)
		return
	}
	list := make(map[string]Info)
	if err := json.Unmarshal(dat, &list); err != nil {
		fmt.Println(err.Error())
		return
	}
	now := time.Now().Unix()
	for k, v := range list {
		if now > v.Last+60*20 {
			continue
		}
		fmt.Println("fastStart:", k, v.Id, v.Proxy)
		workerData := Info{
			room:   k,
			Id:     v.Id,
			Server: v.Server,
			Proxy:  v.Proxy,
			Online: v.Online,
			Start:  v.Start,
			Last:   now,
			Rid:    v.Rid,
			Income: v.Income,
			Dons:   v.Dons,
			Tips:   v.Tips,
		}
		startRoom(workerData)
		time.Sleep(100 * time.Millisecond)
	}
}

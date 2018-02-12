package main

import (
	"net/http"
	"fmt"
	"strconv"
	"io/ioutil"
	"regexp"
	"math/rand"
	"encoding/json"
	"time"
	"encoding/hex"
	"log"
	"github.com/bitly/go-simplejson"
	"bytes"
	"flag"
	"os"
	"github.com/gorilla/websocket"
	"net/url"
)

var (
	cidInfoUrl string = "http://live.bilibili.com/api/player?id=cid:"
	roomId     int    = 0
	id         int    = 0
	chatHost   string = "livecmt-1.bilibili.com"
	serverAddr        = "broadcastlv.chat.bilibili.com:2244"
)

type Client struct {
	Conn *websocket.Conn
}

type jsonMsg struct {
	Info []string          `json:"info"`
	Data map[string]string `json:"data"`
	Cmd  string            `json:"cmd"`
}

func main() {
	mid := flag.Int("id", 0, "房间id")
	flag.Parse()
	id = *mid
	connect := make(chan bool)
	getChatHost()
	chatClient := Client{}
	chatClient.connect()
	go chatClient.getMessage(connect)
	isInTo := chatClient.sendJoinChannel(roomId)
	if isInTo == true {
		fmt.Println("进入房间成功！")
		go chatClient.heartbeat()
	}
	<-connect
}

func getChatHost() {
	resp, err := http.Get("http://api.live.bilibili.com/room/v1/Room/room_init?id=" + strconv.Itoa(id))
	if err != nil {
		fmt.Println("获取ROOMID错误")
		os.Exit(1)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	js, _ := simplejson.NewJson(body)
	roomId, _ = js.Get("data").Get("room_id").Int()
	if roomId == 0 {
		fmt.Println("获取ROOMID错误")
		os.Exit(1)
	}
	resp, err = http.Get(cidInfoUrl + strconv.Itoa(roomId))
	if err != nil {
		fmt.Println("chatHost获取错误")
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)

	reg := regexp.MustCompile(`<server>(.+?)</server>`)
	chatHost = reg.FindStringSubmatch(string(body))[1]
}

func (c *Client) connect() (err error) {

	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/sub"}
	var dialer *websocket.Dialer
	c.Conn, _, err = dialer.Dial(u.String(), nil)
	if err!=nil {
		fmt.Print(err)
		os.Exit(230)
	}
	return nil
}

//进入直播间
func (c *Client) sendJoinChannel(channelId int) bool {
	rand.Seed(int64(time.Now().Nanosecond()))
	random := rand.Float64()
	uid := int(random*200000000000000.0 + 100000000000000.0)
	makeMap := make(map[string]int)
	makeMap["roomid"] = channelId
	makeMap["uid"] = uid
	makeMap["protover"] = 1
	jsonBody, _ := json.Marshal(makeMap)
	body := string(jsonBody)
	handshake := fmt.Sprintf("%08x001000010000000700000001", len(body)+16)

	buf := make([]byte, len(handshake)>>1)
	hex.Decode(buf, []byte(handshake))


	c.Conn.WriteMessage(websocket.BinaryMessage, append(buf, []byte(body)...))

	return true
}

func (c *Client) getMessage(connect chan bool) {
	for {
		_, msg, err := c.Conn.ReadMessage()

		if err != nil {
			log.Println("conn read error:", err)
			connect <- false
			break
		}
		megLEn := MessageSelect(msg, len(msg))
		if len(megLEn) > 2 {
			check := bytes.SplitAfter(megLEn, []byte{0, 0, 0, 5, 0, 0, 0, 0})
			count := len(check)
			for i, v := range check {
				json_content := v
				if i != count-1 {
					cl := len(v) - 16
					json_content = v[:cl]
				}
				MessageType(json_content)
			}
		}
	}

}

func MessageSelect(buf []byte, n int) []byte {
	//bt1:=buf[0:4]
	//bt2:=buf[4:6]
	//bt3:=buf[6:8]
	bt4 := buf[11]
	//bt5:=buf[12:16]
	content := buf[16:n]
	switch int(bt4) {
	case 5:
		return content
	}
	return []byte{0}
}

func MessageType(mesg []byte) {
	json_mesg, _ := simplejson.NewJson(mesg)
	var json_map map[string]interface{}
	json_map, _ = json_mesg.Map()
	cmd := json_map["cmd"]
	switch cmd {
	case "LIVE":
		fmt.Println("直播开始...")
	case "PREPARING":
		fmt.Println("房主准备中...")
	case "DANMU_MSG":
		info := json_map["info"].([]interface{})
		message := info[1]
		postinfo := info[2].([]interface{})
		poster := postinfo[1]
		fmt.Printf("%s say:%s\n", poster, message)
	case "SEND_GIFT":
		data := json_map["data"].(map[string]interface{})
		num := data["num"].(json.Number)
		numfloat, _ := strconv.ParseFloat(string(num), 64)
		giftName := data["giftName"]
		uname := data["uname"]
		action := data["action"]
		price := data["price"].(json.Number)
		pricefloat, _ := strconv.ParseFloat(string(price), 64)
		count_price := int(numfloat) * int(pricefloat)
		fmt.Printf("%s%s%s个%s,价值%d\n", uname, action, num, giftName, count_price)
	case "WELCOME":
		data := json_map["data"].(map[string]interface{})
		user := data["uname"]
		fmt.Printf("欢迎 %s 进入直播间\n", user)
	}
}
func (c *Client) heartbeat() {
	for {
		buf := make([]byte, 16)
		hex.Decode(buf, []byte("0000001f001000010000000200000001"))
		c.Conn.WriteMessage(websocket.BinaryMessage, buf)
		time.Sleep(30 * time.Second)
	}
}

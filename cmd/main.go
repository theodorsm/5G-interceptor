package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/http2"
	"gopkg.in/yaml.v2"
)

type TestcaseConfig struct {
	Supi      string     `yaml:supi`
	Testcases []Testcase `yaml:testcases`
}

type TestcaseResult struct {
	ResponseType byte     `bson:"response_type"`
	Testcase     Testcase `bson:"test_case"`
}

type Testcase struct {
	Id        uint     `yaml:id`
	Result    bool     `yaml:result`
	Plain     bool     `yaml:plain`
	Integrity *string  `yaml:integrity`
	Ciphering *string  `yaml:ciphering`
	Mac       string   `yaml:mac`
	MsgType   uint     `yaml:"msg_type"`
	Offsets   []Offset `yaml:offsets`
}

type OffsetTestcase struct {
	Id      uint     `yaml:id`
	Result  bool     `yaml:result`
	Plain   bool     `yaml:plain`
	MsgType uint     `yaml:"msg_type"`
	Offsets []Offset `yaml:offsets`
}

type Offset struct {
	Offset uint   `yaml:offset`
	Value  string `yaml:value`
}

var config TestcaseConfig

var SERVER_HOST = "127.0.0.1"
var SERVER_PORT = "1337"
var AMF_SBI_ADDR = "127.0.0.5:7777"

var state string
var currentCase int
var SUPI_STATE = "imsiState"
var MSG_TYPE_STATE = "msgTypeState"
var MSG_STATE = "msgState"
var RES_STATE = "resState"
var FALSE byte = 0x0
var TRUE byte = 0x1
var ENC byte = 0x2

var collection *mongo.Collection
var ctx = context.TODO()

func init_mongo() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017/")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	collection = client.Database("5g-tester").Collection("testcases")
}

func main() {
	init_mongo()
	data, err := ioutil.ReadFile("testcases.yaml")
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Println(config)

	lenTestcases := len(config.Testcases)

	fmt.Println("Number of testcases: " + fmt.Sprint(lenTestcases))

	state = SUPI_STATE
	currentCase = 0

	fmt.Println("Server Running...")

	server, err := net.Listen("tcp", SERVER_HOST+":"+SERVER_PORT)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer server.Close()
	fmt.Println("Listening on " + SERVER_HOST + ":" + SERVER_PORT)
	fmt.Println("Waiting for client...")
	go sendTestcaseEnable(false)
	go sendTestcaseEnable(true)

	for currentCase < lenTestcases {
		connection, err := server.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		fmt.Println("client connected")
		processClient(connection)
	}
	fmt.Println("done")
}

func processClient(conn net.Conn) {

	defer func() {
		fmt.Println("Closing connection")
		conn.Close()
	}()

	if currentCase >= len(config.Testcases) {
		return
	}

	fmt.Println("Current state: ", state)
	fmt.Println("Current case: ", currentCase)
	switch state {
	case SUPI_STATE:
		_, err := conn.Write([]byte(config.Supi))
		if err != nil {
			fmt.Println("Error sending: ", err.Error())
		}
		state = MSG_TYPE_STATE
		break
	case MSG_TYPE_STATE:
		buffer := make([]byte, 1)
		_, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("Error reading:", err.Error())
		}
		fmt.Println("Msg type: ", buffer[0])
		if byte(getCurrentCase().MsgType) == buffer[0] {
			if getCurrentCase().Plain {
				_, err := conn.Write([]byte{TRUE})
				if err != nil {
					fmt.Println("Error sending: ", err.Error())
				}
			} else {
				_, err := conn.Write([]byte{ENC})
				if err != nil {
					fmt.Println("Error sending: ", err.Error())
				}
			}
			state = MSG_STATE
			break
		}
		_, err = conn.Write([]byte{FALSE})
		if err != nil {
			fmt.Println("Error sending: ", err.Error())
		}
		break
	case MSG_STATE:
		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)

		hexBuff := hex.EncodeToString(buffer[:n])
		fmt.Printf("OG message:  %v\n", hexBuff)

		if err != nil {
			fmt.Println("Error reading:", err.Error())
		}
		if getCurrentCase().Ciphering != nil {
			hexBuff = hexBuff[:20] + *getCurrentCase().Ciphering + hexBuff[21:]
		}
		if getCurrentCase().Integrity != nil {
			hexBuff = hexBuff[:21] + *getCurrentCase().Integrity + hexBuff[22:]
		}

		mac := getCurrentCase().Mac
		if len(mac) == 8 {
			hexBuff = hexBuff[:4] + mac + hexBuff[12:]
		}
		for _, o := range getCurrentCase().Offsets {
			fmt.Printf("Offset %v\n", o)
			hexBuff = hexBuff[:o.Offset] + o.Value + hexBuff[o.Offset+uint(len(o.Value)):]
		}

		fmt.Printf("MOD message: %v\n", hexBuff)
		msgBuff, err := hex.DecodeString(hexBuff)
		if err != nil {
			fmt.Println("Hexbuff of wrong length: ", err.Error())
		}
		_, err = conn.Write(msgBuff)
		if err != nil {
			fmt.Println("Error sending: ", err.Error())
		}
		state = RES_STATE
		break
	case RES_STATE:
		buffer := make([]byte, 1)
		_, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("Error reading:", err.Error())
		}
		fmt.Println("Response msg type from testcase: ", buffer[0])

		result := TestcaseResult{ResponseType: buffer[0], Testcase: getCurrentCase()}
		_, err = collection.InsertOne(ctx, result)
		if err != nil {
			fmt.Println("Error insterting result in mongodb:", err.Error())
		}
		_, err = conn.Write([]byte{FALSE})
		if err != nil {
			fmt.Println("Error sending: ", err.Error())
		}

		time.Sleep(5 * time.Second)

		url := fmt.Sprintf("http://%s/namf-callback/v1/%s/dereg-notify", AMF_SBI_ADDR, config.Supi)
		var jsonData = []byte(`{
			"deregReason": "REREGISTRATION_REQUIRED",
			"accessType": "3GPP_ACCESS"
		}`)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Fatalf("impossible to build request: %s", err)
		}
		req.Header.Set("Content-Type", "application/json;")

		// Create an HTTP/2 Transport with Prior Knowledge
		transport := &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		}

		client := &http.Client{Transport: transport}
		_, err = client.Do(req)
		currentCase++
		state = MSG_TYPE_STATE
		if currentCase < len(config.Testcases) {
			sendTestcaseEnable(true)
		}
		break
	default:
		break
	}
	return
}

func getCurrentCase() Testcase {
	return config.Testcases[currentCase]
}

func sendTestcaseEnable(b bool) {
	var url string
	if b {
		url = fmt.Sprintf("http://%s/testcase-enable/v1/true", AMF_SBI_ADDR)
	} else {
		url = fmt.Sprintf("http://%s/testcase-enable/v1/false", AMF_SBI_ADDR)
	}
	req, err := http.NewRequest("GET", url, bytes.NewBuffer([]byte{}))
	if err != nil {
		log.Fatalf("impossible to build request: %s", err)
	}
	// Create an HTTP/2 Transport with Prior Knowledge
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}

	client := &http.Client{Transport: transport}
	_, err = client.Do(req)
	return
}

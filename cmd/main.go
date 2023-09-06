package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"golang.org/x/net/http2"

	"github.com/google/uuid"
	"gopkg.in/yaml.v2"
)

type TestcaseConfig struct {
	Nrf       string    `yaml:nrf`
	Imsi      string    `yaml:imsi`
	Testcases Testcases `yaml:testcases`
}

type Testcases struct {
	SecCmdModeCases []SecCmdModeTestcase `yaml:"sec_cmd_mode_testcases"`
	OffsetCases     []OffsetTestcase     `yaml:"offset_testcases"`
}

type SecCmdModeTestcase struct {
	Id        uint   `yaml:id`
	Result    bool   `yaml:result`
	Plain     bool   `yaml:plain`
	Integrity byte   `yaml:integrity`
	Ciphering byte   `yaml:ciphering`
	Mac       string `yaml:mac`
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
var localAddr string
var localPort string

func main() {
	localAddr = "127.0.13.37"
	localPort = ":7777"
	data, err := ioutil.ReadFile("testcases.yaml")
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Println(config)

	registerToNrf()
	http.HandleFunc("/ninf/v1/config", handleConfigRequest)
	http.HandleFunc("/ninf/v1/imsi", handleImsiRequest)
	log.Fatal(http.ListenAndServe(localAddr+localPort, nil))
}

func registerToNrf() {
	id := uuid.New()
	httpurl := fmt.Sprintf("http://%s/nnrf-nfm/v1/nf-instances/%s", config.Nrf, id.String())
	fmt.Println("HTTP JSON URL:", httpurl)

	type NfRegister struct {
		NfInstanceId  string   `json:"nfInstanceId"`
		NfType        string   `json:"nfType"`
		NfStatus      string   `json:"nfStatus"`
		Ipv4addresses []string `json:"ipv4Addresses"`
		/*
			AllowedNfTypes             []string `json:"allowedNfTypes"`
			Priority                   uint     `json:"priority"`
			Capacity                   uint     `json:"capacity"`
			Load                       uint     `json:"load"`
			NfProfileChangesSupportInd bool     `json:"nfProfileChangesSupportInd"`
		*/
	}

	//jjsonData := NfRegister{id.String(), "INF", "REGISTERED", []string{localAddr}, []string{"AMF"}, 0, 100, 0, true}
	jsonData := NfRegister{id.String(), "CUSTOM_INF", "REGISTERED", []string{localAddr}}

	marshalled, err := json.Marshal(jsonData)
	fmt.Println(string(marshalled))
	if err != nil {
		log.Fatalf("impossible to marshall jsonData: %s", err)
	}
	req, err := http.NewRequest("PUT", httpurl, bytes.NewReader(marshalled))
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

	res, err := client.Do(req)
	if err != nil {
		log.Fatalf("impossible to send request: %s", err)
	}
	log.Printf("status Code: %d", res.StatusCode)
	defer res.Body.Close()
	// read body
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("impossible to read all body of response: %s", err)
	}
	log.Printf("res body: %s", string(resBody))

}

func handleConfigRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	response := config

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func handleImsiRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	type ImsiResponse struct {
		Imsi string `json:"imsi"`
	}

	response := ImsiResponse{Imsi: config.Imsi}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

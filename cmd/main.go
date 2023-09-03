package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"gopkg.in/yaml.v2"
)

type Response struct {
	Message string `json:"message"`
}

type TestcaseConfig struct {
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

func main() {
	http.HandleFunc("/ninf/v1/config", handleRequest)
	log.Fatal(http.ListenAndServe("127.0.13.37:7777", nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := ioutil.ReadFile("testcases.yaml")
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	var config TestcaseConfig

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Println(config)
	response := config

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

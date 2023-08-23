package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"gopkg.in/yaml.v3"
)

type AlertConfig struct {
	ElkHost           string   `yaml:"elk_host"`
	ElkUsername       string   `yaml:"elk_username"`
	ElkPassword       string   `yaml:"elk_password"`
	ElkIndex          string   `yaml:"elk_index"`
	ElkThreshold      uint64   `yaml:"elk_threshold"`
	Query             string   `yaml:"elk_query"`
	Whitelist         []string `yaml:"whitelist"`
	SlackWebhook      string   `yaml:"slack_webhook"`
	SlackMessageTitle string   `yaml:"slack_message_title"`
}

func ReadYaml(filepath string) *AlertConfig {
	yamlFileReader, err := os.Open(filepath)
	if err != nil {
		log.Fatalf("Error Reading file: %s. Error:%s", filepath, err)
		return &AlertConfig{}
	}
	defer yamlFileReader.Close()
	// load yaml file
	data, err := io.ReadAll(yamlFileReader)
	if err != nil {
		fmt.Println(err)
		return &AlertConfig{}
	}

	// load config from yaml data
	config := AlertConfig{}
	err = yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		log.Fatalf("Error:%s", err)
		return &AlertConfig{}
	}

	// fmt.Println(config.ElkHost)
	// fmt.Println(config.ElkUsername)
	// fmt.Println(config.ElkPassword)
	// fmt.Println(config.ElkIndex)
	// fmt.Println(config.ElkThreshold)
	// fmt.Println(config.Query)
	// fmt.Println(config.Whitelist)

	return &config
}

func parseIPs(ipStrings []string) ([]net.IP, error) {
	var ips []net.IP
	for _, ipStr := range ipStrings {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", ipStr)
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

func isInIPWhitelist(ipStr string, whitelist []net.IP) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, allowedIP := range whitelist {
		if ip.Equal(allowedIP) {
			return true
		}
	}
	return false
}

func sendSlackMessage(webhookURL string, message map[string]string) error {
	// Convert the message payload to JSON
	jsonPayload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Create an HTTP POST request
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Create an HTTP client and send the request
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("%d received instead of 200", resp.StatusCode)
	}

	return nil
}

func main() {
	// Read the YAML file
	elkConfig := ReadYaml("test.yaml")

	cfg := elasticsearch.Config{
		Addresses: []string{
			elkConfig.ElkHost,
		},
		Username: elkConfig.ElkUsername,
		Password: elkConfig.ElkPassword,
	}

	es, _ := elasticsearch.NewClient(cfg)

	res, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(elkConfig.ElkIndex),
		es.Search.WithBody(strings.NewReader(elkConfig.Query)),
	)
	if err != nil {
		log.Fatalf("Error performing search: %s", err)
	}
	// fmt.Println(res.String())
	defer res.Body.Close()

	if res.IsError() {
		log.Fatalf("Elasticsearch error response: %s", res.String())
	}

	// parse json from response body
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Fatalf("Error parsing JSON response: %s", err)
	}

	// Now 'result' contains the parsed JSON data
	// fmt.Println("Parsed JSON response:", result)

	// get white listed ips
	ipWhitelist, _ := parseIPs(elkConfig.Whitelist)

	aggs := result["aggregations"].(map[string]interface{})
	clientIPs := aggs["aggs_data"].(map[string]interface{})
	buckets := clientIPs["buckets"].([]interface{})

	resultStr := ""
	for _, item := range buckets {
		clientIPDetails := item.(map[string]interface{})
		count := clientIPDetails["doc_count"].(float64)
		clientIP := clientIPDetails["key"].(string)
		if count > float64(elkConfig.ElkThreshold) && !isInIPWhitelist(clientIP, ipWhitelist) {
			resultStr += fmt.Sprintf("%s %s\n", strconv.FormatFloat(count, 'f', -1, 64), clientIP)
		}
	}
	if resultStr != "" {
		resultStr = string(elkConfig.SlackMessageTitle) + "\n" + resultStr
	} else {
		fmt.Println("No Data Found")
	}

	// send slack message if webhook is available
	if len(elkConfig.SlackWebhook) > 4 {
		message := map[string]string{
			"type": "mrkdwn",
			"text": resultStr,
		}
		sendSlackMessage(elkConfig.SlackWebhook, message)
	}
}

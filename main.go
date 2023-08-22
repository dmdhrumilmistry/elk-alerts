package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/joho/godotenv"
)

type Bucket struct {
	DocCount int    `json:"doc_count"`
	Key      string `json:"key"`
}

func main() {
	// load .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Some error occured while loading .env file. Err: %s", err)
	}

	// elk config
	elkHost := os.Getenv("ELK_HOST")
	elkUser := os.Getenv("ELK_USERNAME")
	elkPassword := os.Getenv("ELK_PASSWORD")
	index := os.Getenv("ELK_INDEX") // replace with index name
	threshold := os.Getenv("THRESHOLD")
	if threshold == "" {
		threshold = "0"
	}
	thr, err := strconv.ParseInt(threshold, 10, 64)
	if err != nil {
		log.Fatalf("Invalid Threshold. Error: %s", err)
	}

	cfg := elasticsearch.Config{
		Addresses: []string{
			elkHost,
		},
		Username: elkUser,
		Password: elkPassword,
	}

	es, _ := elasticsearch.NewClient(cfg)

	query := `{
		"query": {
		  "bool": {
			"filter": [
			  {
				"range": {
				  "@timestamp": {
					"gte": "now-60m"
				  }
				}
			  },
			  {
				"term": {
				  "response.keyword": {
					"value": 404 
				  }
				}
			  }
			]
		  }
		},
		"size": 0,
		"aggs": {
		  "aggs_data": {
			"terms": {
			  "field": "client_ip.keyword"
			}
		  }
		}
	  }`

	res, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(index),
		es.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		log.Fatalf("Error performing search: %s", err)
	}
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

	aggs := result["aggregations"].(map[string]interface{})
	clientIPs := aggs["aggs_data"].(map[string]interface{})
	buckets := clientIPs["buckets"].([]interface{})

	resultStr := ""
	for _, item := range buckets {
		clientIPDetails := item.(map[string]interface{})
		count := clientIPDetails["doc_count"].(float64)
		clientIP := clientIPDetails["key"].(string)
		if count > float64(thr) {
			resultStr += fmt.Sprintf("%s %s\n", strconv.FormatFloat(count, 'f', -1, 64), clientIP)
		}
	}
	fmt.Println(resultStr)
}

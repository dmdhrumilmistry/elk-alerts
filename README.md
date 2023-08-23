# ELK alerts

Simple Alerting tool which queries elasticsearch for data, filters it and sends alerts on slack.

## Installation

### Using GO Install

```bash
go install github.com/dmdhrumilmistry/elk-alerts@latest
```

### Using git clone

```bash
git clone https://github.com/dmdhrumilmistry/elk-alerts.git
cd elk-alerts
go install
```

## Usage

- Basic Usage

    ```bash
    elk-alerts -f config.yaml
    ```

- Set crontab for periodic alerts

## Example Config file

```yaml
# this can help to detect directory bruteforcing
elk_host: http://localhost:9200
elk_username: elk_alerts
elk_password: 'your_super_secure_password'
elk_index: 'your-index-*'
elk_threshold: 100
elk_query: |
  {
    "query": {
      "bool": {
        "filter": [
          {
            "range": {
              "@timestamp": {
                "gte": "now-5m"
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
  }

# aggs must contain aggs_data
whitelist: ['1.1.1.1','1.0.0.1']

# slack webhook configs
slack_webhook: https://hooks.slack.com/services/your/slack/webhook
slack_message_title: "*Test Message* :bomb:"
```

- `elk_alerts` must have read only permission to work.

- replace `elk_query` param with query from elk devtools console.

- Tool provide option to whitelist ips from alerts.

- `aggs` must have `aggs_data` key in order to work correctly.

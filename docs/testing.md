# Testing 

To test event querying you need an Elasticsearch server

### Elasticsearch

The easiest solution for an Elasticsearch server is to use docker.
```
docker run -d -p 9200:9200 --name elasticsearch elasticsearch:2.6 
```

### load fake data
```
./scripts/load_events.sh --addr localhost:9200 --org 1 --endpoint google_com --probe new-york --type http --count 10
```

### run worldping-gw
```
worldping-gw -elasticsearch-url=http://localhost:9200 -admin-key=secret -addr :8080
```

### start grafana
```
docker run -d --name grafana -p 3000:3000 grafana/grafana
```

### add datasource to grafana

You can now add a datasource to grafana with the following settings
```
Name: worldping-gw
Type: ElasticSearch
Url: http://172.17.0.1:8080/elasticsearch
Access: direct
BasicAuth: true
User: api_key
password: secret

Index name: [events-]YYYY-MM-DD  
Pattern: Daily
time field name: timestamp
Version: 1.x
```

### build dashboards.
You can now build dashboards in grafana using the tsdbgw datasource.




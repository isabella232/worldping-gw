package elasticsearch

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestElasticsearch(t *testing.T) {
	orgID := int(1234)

	rawSearchHeader := []byte(`{
	"search_type":"count",
	"ignore_unavailable":true,
	"index":["events-2016-12-26","events-2016-12-27","events-2016-12-28","events-2016-12-29","events-2016-12-30","events-2016-12-31","events-2017-01-01","events-2017-01-02"]
}`)

	rawSearchHeaderInvalidIndex := []byte(`{
	"search_type":"count",
	"ignore_unavailable":true,
	"index":["events-2016-12-2","events-2016-12-27","events-2016-12-28","events-2016-12-29","events-2016-12-30","events-2016-12-31","events-2017-01-01","events-2017-01-02"]
}`)

	rawSearch := []byte(`{
	"size":0,
	"query":{
		"bool":{
			"must":[
				{
					"range":{
						"timestamp":{
							"gte":"1482791304618",
							"lte":"1483396104619",
							"format":"epoch_millis"
						}
					}
				},
				{
					"query_string":{
						"analyze_wildcard":true,
						"query":"tags.endpoint:(\"\\~google_com_demo\") AND tags.collector:(\"amsterdam\") AND tags.monitor_type:(\"dns\") AND severity:(\"ERROR\" OR \"OK\")"
					}
				}
			]
		}
	},
	"aggs":{
		"3":{
			"terms":{
				"field":"severity",
				"size":500,
				"order":{
					"_term":"asc"
				}
			},
			"aggs":{
				"2":{
					"date_histogram":{
						"interval":"6h",
						"field":"timestamp",
						"min_doc_count":0,
						"extended_bounds":{
							"min":"1482791304618",
							"max":"1483396104619"
						},
						"format":"epoch_millis"
					},
					"aggs":{}
				}
			}
		}
	}
}`)

	Convey("When validating a valid search header", t, func() {
		err := validateHeader(rawSearchHeader)
		So(err, ShouldBeNil)
	})

	Convey("When validating a search header with an invalid index", t, func() {
		err := validateHeader(rawSearchHeaderInvalidIndex)
		So(err, ShouldResemble, fmt.Errorf("invalid index name. %s", "events-2016-12-2"))
	})

	Convey("When transforming a search request", t, func() {
		transformed, err := transformSearch(orgID, rawSearch)
		So(err, ShouldBeNil)
		So(string(transformed), ShouldEqual, `{"size":0,"query":{"bool":{"must":[{"bool":{"must":[{"range":{"timestamp":{"gte":"1482791304618","lte":"1483396104619"}}},{"query_string":{"analyze_wildcard":true,"query":"tags.endpoint:(\"\\~google_com_demo\") AND tags.collector:(\"amsterdam\") AND tags.monitor_type:(\"dns\") AND severity:(\"ERROR\" OR \"OK\")"}}]}}],"filter":{"term":{"org_id":1234}}}},"aggs":{"3":{"aggs":{"2":{"aggs":{},"date_histogram":{"extended_bounds":{"max":"1483396104619","min":"1482791304618"},"field":"timestamp","interval":"6h","min_doc_count":0}}},"terms":{"field":"severity","order":{"_term":"asc"},"size":500}}}}`)
	})
}

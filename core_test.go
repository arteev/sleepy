package sleepy

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"
)

type Item struct{}

func (item Item) Get(values url.Values, headers http.Header) (int, interface{}, http.Header) {
	items := []string{"item1", "item2"}
	data := map[string][]string{"items": items}
	return 200, data, nil
}

func TestBasicGet(t *testing.T) {
	item := new(Item)
	var api = NewAPI()
	api.AddResource(item, "/items", "/bar", "/baz")
	go func() {
		api.Start(3000)
	}()
	<-time.After(500 * time.Millisecond)
	resp, err := http.Get("http://localhost:3000/items")
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if string(body) != "{\n  \"items\": [\n    \"item1\",\n    \"item2\"\n  ]\n}" {
		t.Error("Not equal.")
		t.Log(string(body))
	}

}

type ItemCT struct {
	contentType http.Header
}
type DataExport struct {
	Name string
}

func (item ItemCT) Get(r *http.Request) (int, interface{}, http.Header) {
	items := []DataExport{
		{"Name1"},
		{"Name2"},
	}
	return 200, items, item.contentType
}

func TestContentType(t *testing.T) {
	item := new(ItemCT)
	var api = NewAPI()
	api.AddResource(item, "/items", "/bar", "/baz")
	go func() {
		api.Start(3002,
			WithMarshaler("application/xml", xml.MarshalIndent),
		)
	}()
	<-time.After(500 * time.Millisecond)
	cases := []struct {
		Contenttype http.Header
		Result      string
	}{
		{nil, "[\n  {\n    \"Name\": \"Name1\"\n  },\n  {\n    \"Name\": \"Name2\"\n  }\n]"},
		{map[string][]string{"Content-type": {"application/json"}}, "[\n  {\n    \"Name\": \"Name1\"\n  },\n  {\n    \"Name\": \"Name2\"\n  }\n]"},
		{map[string][]string{"Content-type": {"application/xml"}}, "<DataExport>\n  <Name>Name1</Name>\n</DataExport>\n<DataExport>\n  <Name>Name2</Name>\n</DataExport>"},
		{map[string][]string{"Content-type": {"application/xml;charset=utf-8"}}, "<DataExport>\n  <Name>Name1</Name>\n</DataExport>\n<DataExport>\n  <Name>Name2</Name>\n</DataExport>"},
	}

	for _, c := range cases {
		item.contentType = c.Contenttype
		resp, err := http.Get("http://localhost:3002/items")
		if err != nil {
			t.Error(err)
		}
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != c.Result {
			ct, ok := c.Contenttype["Content-type"]
			if !ok {
				ct = []string{"<nil>"}
			}
			t.Errorf("Not equal for Content-type:%s", ct[0])
		}
	}

}

var once sync.Once

func prepareBenchmark(b *testing.B) {
	once.Do(func() {
		item := new(Item)
		var api = NewAPI()
		api.AddResource(item, "/items", "/bar", "/baz")
		go func() {
			api.Start(3003)
		}()
	})
}

func BenchmarkBasicGet(b *testing.B) {
	prepareBenchmark(b)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		resp, err := http.Get("http://localhost:3003/items")
		if err != nil {
			b.Error(err)
		}
		if err := resp.Body.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

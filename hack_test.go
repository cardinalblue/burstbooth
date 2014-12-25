package hack

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"hack20141225/aws"
	"hack20141225/util"
)

func TestPostAndVote(t *testing.T) {
	setup(t)
	ts := httptest.NewServer(http.DefaultServeMux)
	defer ts.Close()

	v := url.Values{"url": {"http://127.0.0.1/a.jpg"}}
	util.JSONReq3("POST", ts.URL+"/PostImg?"+v.Encode(), nil)
	imgs := []PostDDB{}
	util.JSONReq3("GET", ts.URL+"/Hot", &imgs)
	if imgs[0].S.N != "0" {
		t.Fatalf("wrong imgs")
	}

	// Vote twice with the same deviceID, the score should still be 1.
	v = url.Values{"device_id": {"ddd"}, "key": {base64.StdEncoding.EncodeToString(imgs[0].K.B)}}
	util.JSONReq3("POST", ts.URL+"/Vote?"+v.Encode(), nil)
	util.JSONReq3("POST", ts.URL+"/Vote?"+v.Encode(), nil)
	imgs = []PostDDB{}
	util.JSONReq3("GET", ts.URL+"/Hot", &imgs)
	if imgs[0].S.N != "1" {
		t.Fatalf("wrong imgs")
	}
}

func TestHotPaginate(t *testing.T) {
	setup(t)
	ts := httptest.NewServer(http.DefaultServeMux)
	defer ts.Close()

	postAndVoteNTimes(ts, "http://127.0.0.1/3votes.jpg", 3)
	postAndVoteNTimes(ts, "http://127.0.0.1/2votes.jpg", 2)
	postAndVoteNTimes(ts, "http://127.0.0.1/1vote.jpg", 1)

	// Paginate downwards
	imgs := []PostDDB{}
	util.JSONReq3("GET", ts.URL+"/Hot?limit=2", &imgs)
	if imgs[0].URL.S != "http://127.0.0.1/3votes.jpg" {
		t.Fatalf("")
	}
	if imgs[1].URL.S != "http://127.0.0.1/2votes.jpg" {
		t.Fatalf("")
	}
	v := url.Values{"key": {base64.StdEncoding.EncodeToString(imgs[1].K.B)}, "score": {imgs[1].S.N}}
	imgs2 := []PostDDB{}
	util.JSONReq3("GET", ts.URL+"/Hot?"+v.Encode(), &imgs2)
	if imgs2[0].URL.S != "http://127.0.0.1/1vote.jpg" {
		t.Fatalf("")
	}

	// Paginate upwards
	postAndVoteNTimes(ts, "http://127.0.0.1/4votes.jpg", 4)
	v = url.Values{"key": {base64.StdEncoding.EncodeToString(imgs[0].K.B)}, "score": {imgs[0].S.N}, "forward": {"true"}}
	imgs = []PostDDB{}
	util.JSONReq3("GET", ts.URL+"/Hot?"+v.Encode(), &imgs)
	if imgs[0].URL.S != "http://127.0.0.1/4votes.jpg" {
		t.Fatalf("")
	}
}

func postAndVoteNTimes(ts *httptest.Server, imgurl string, voteNum int) {
	v := url.Values{"url": {imgurl}}
	p := PostDDB{}
	util.JSONReq3("POST", ts.URL+"/PostImg?"+v.Encode(), &p)
	for i := 0; i < voteNum; i++ {
		v := url.Values{"device_id": {fmt.Sprintf("%d", i)}, "key": {base64.StdEncoding.EncodeToString(p.K.B)}}
		util.JSONReq3("POST", ts.URL+"/Vote?"+v.Encode(), nil)
	}
}

func setup(t *testing.T) {
	resp := struct {
		LastEvaluatedTableName string
		TableNames             []string
	}{}
	err := aws.DynamoDBPostBytes("ListTables", []byte("{}"), &resp)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for i := 0; i < len(resp.TableNames); i++ {
		req := struct{ TableName string }{TableName: resp.TableNames[i]}
		err = aws.DynamoDBPost("DeleteTable", &req, nil)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	CreateDDBTables()
}

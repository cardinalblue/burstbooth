package burstbooth

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/cardinalblue/burstbooth/aws"
)

const (
	postTypeGIF = "gif"
)

type PostDDB struct {
	I   struct{ S string } // just an index
	K   struct{ B []byte } // a unique key for this post
	S   struct{ N string } // score
	URL struct{ S string } // url of the image

	// Optional Attributes
	C *struct{ S string } `json:",omitempty"` // caption
}

func postPK(index string, key []byte) []byte {
	b := append([]byte(index), '\x00')
	b = append(b, key...)
	return b
}

type PostJSON struct {
	I   struct{ S string }
	K   struct{ B []byte }
	S   struct{ N string }
	URL struct{ S string }

	C struct{ S string }

	V bool
}

func postDDBToJSON(p PostDDB) PostJSON {
	pj := PostJSON{}
	pj.I = p.I
	pj.K = p.K
	pj.S = p.S
	pj.URL = p.URL
	if p.C != nil {
		pj.C.S = p.C.S
	}
	return pj
}

// postJSONByKDesc implements sort.Interface for PostJSON.
type postJSONByKDesc []PostJSON

func (a postJSONByKDesc) Len() int      { return len(a) }
func (a postJSONByKDesc) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a postJSONByKDesc) Less(i, j int) bool {
	is, _ := strconv.Atoi(a[i].S.N)
	js, _ := strconv.Atoi(a[j].S.N)
	if is != js {
		return is > js
	}
	return bytes.Compare(a[i].K.B, a[j].K.B) > 0
}

type VoteDDB struct {
	D struct{ B []byte } // device ID
	P struct{ B []byte } // post ID
}

var (
	ddbTablePost = os.Getenv("DDB_TABLE_POST")
	ddbTableVote = os.Getenv("DDB_TABLE_VOTE")
)

func init() {
	jsonAPI("/PostImg", PostImg)
	jsonAPI("/Hot", Hot)
	jsonAPI("/Vote", Vote)
	http.HandleFunc("/", root)
}

// PostImg posts an image URL to the server
//   curl 'http://localhost:8080/PostImg?url=http%3A%2F%2F127.0.0.1%2Fa.jpg'
func PostImg(w http.ResponseWriter, r *http.Request) *appError {
	url := r.FormValue("url")
	caption := r.FormValue("caption")

	t := time.Now().UnixNano()
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.BigEndian, t); err != nil {
		return &appError{Message: err.Error(), Code: http.StatusInternalServerError}
	}
	post := PostDDB{}
	post.I.S = postTypeGIF
	post.K.B = buf.Bytes()
	post.S.N = "0"
	post.URL.S = url
	if caption != "" {
		post.C = &struct{ S string }{S: caption}
	}
	bodyj := struct {
		TableName                 string
		Item                      PostDDB
		ConditionExpression       string
		ExpressionAttributeValues struct {
			I struct{ S string } `json:":i"`
			K struct{ B []byte } `json:":k"`
		}
	}{}
	bodyj.TableName = ddbTablePost
	bodyj.Item = post
	bodyj.ConditionExpression = "I <> :i and K <> :k"
	bodyj.ExpressionAttributeValues.I.S = postTypeGIF
	bodyj.ExpressionAttributeValues.K.B = post.K.B
	if err := aws.DynamoDBPost("PutItem", bodyj, nil); err != nil {
		glog.Errorf("%v", err)
		return &appError{Message: err.Error(), Code: http.StatusInternalServerError}
	}
	json.NewEncoder(w).Encode(post)
	return nil
}

// Hot returns the hottest images.
//  curl http://localhost:8080/Hot?device_id=ddd
func Hot(w http.ResponseWriter, r *http.Request) *appError {
	var key []byte = nil
	var score int
	if keyStr := r.FormValue("key"); keyStr != "" {
		k, err := base64.StdEncoding.DecodeString(r.FormValue("key"))
		if err != nil {
			return &appError{Message: err.Error(), Code: http.StatusBadRequest}
		}
		key = k
		score, err = strconv.Atoi(r.FormValue("score"))
		if err != nil {
			return &appError{Message: err.Error(), Code: http.StatusBadRequest}
		}
	}
	forward := false
	if r.FormValue("forward") == "true" {
		forward = true
	}
	limit := 20
	if limitStr := r.FormValue("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil {
			return &appError{Message: err.Error(), Code: http.StatusBadRequest}
		}
		limit = l
	}
	deviceID := []byte(r.FormValue("device_id"))

	posts, err := getPostsByScore(postTypeGIF, key, score, forward, limit)
	if err != nil {
		glog.Errorf("%v", err)
		return &appError{Message: err.Error(), Code: http.StatusInternalServerError}
	}
	c := make(chan PostJSON)
	var wg sync.WaitGroup
	wg.Add(len(posts))
	for _, p := range posts {
		go func(p PostDDB) {
			defer wg.Done()
			pj := postDDBToJSON(p)
			if len(deviceID) > 0 {
				bodyj := struct {
					TableName string
					Key       struct {
						D struct{ B []byte }
						P struct{ B []byte }
					}
				}{}
				bodyj.TableName = ddbTableVote
				bodyj.Key.D.B = deviceID
				bodyj.Key.P.B = postPK(postTypeGIF, p.K.B)
				v := struct{ Item *VoteDDB }{}
				if err := aws.DynamoDBPost("GetItem", bodyj, &v); err != nil {
					glog.Errorf("%v", err)
				} else {
					if v.Item != nil {
						pj.V = true
					}
				}
			}
			c <- pj
		}(p)
	}
	go func() {
		wg.Wait()
		close(c)
	}()
	resp := struct {
		Posts []PostJSON
	}{}
	for pj := range c {
		resp.Posts = append(resp.Posts, pj)
	}
	sort.Sort(postJSONByKDesc(resp.Posts))

	json.NewEncoder(w).Encode(resp)
	return nil
}

// Vote votes for an image.
//   curl 'http://localhost:8080/Vote?device_id=ddd&key=E7MySUSwyFQ%3D'
func Vote(w http.ResponseWriter, r *http.Request) *appError {
	deviceID := r.FormValue("device_id")
	if deviceID == "" {
		return &appError{Message: "no device_id", Code: http.StatusBadRequest}
	}
	key, err := base64.StdEncoding.DecodeString(r.FormValue("key"))
	if err != nil {
		return &appError{Message: err.Error(), Code: http.StatusBadRequest}
	}

	vote := VoteDDB{}
	vote.D.B = []byte(deviceID)
	vote.P.B = postPK(postTypeGIF, key)
	bodyj := struct {
		TableName                 string
		Item                      VoteDDB
		ConditionExpression       string
		ExpressionAttributeValues struct {
			D struct{ B []byte } `json:":d"`
			P struct{ B []byte } `json:":p"`
		}
	}{}
	bodyj.TableName = ddbTableVote
	bodyj.Item = vote
	bodyj.ConditionExpression = "D <> :d and P <> :p"
	bodyj.ExpressionAttributeValues.D.B = vote.D.B
	bodyj.ExpressionAttributeValues.P.B = vote.P.B
	if err := aws.DynamoDBPost("PutItem", bodyj, nil); err != nil {
		if derr, ok := err.(*aws.ErrDynamoDB); ok && derr.Type == "ConditionalCheckFailedException" {
			return &appError{Message: derr.Error(), Code: http.StatusBadRequest}
		}
		glog.Errorf("%v", err)
		return &appError{Message: err.Error(), Code: http.StatusInternalServerError}
	}

	bj := struct {
		TableName string
		Key       struct {
			I struct{ S string }
			K struct{ B []byte }
		}
		UpdateExpression          string
		ExpressionAttributeValues struct {
			S struct{ N string } `json:":s"`
		}
		ReturnValues string
	}{}
	bj.TableName = ddbTablePost
	bj.Key.I.S = postTypeGIF
	bj.Key.K.B = key
	bj.UpdateExpression = "ADD S :s"
	bj.ExpressionAttributeValues.S.N = "1"
	bj.ReturnValues = "ALL_NEW"
	ur := struct{ Attributes PostDDB }{}
	if err := aws.DynamoDBPost("UpdateItem", bj, &ur); err != nil {
		glog.Errorf("%v", err)
	}

	pj := postDDBToJSON(ur.Attributes)
	pj.V = true
	json.NewEncoder(w).Encode(pj)
	return nil
}

func root(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello world!"))
}

func getPostsByScore(postType string, key []byte, score int, forward bool, limit int) ([]PostDDB, error) {
	var body []byte
	if key == nil {
		bodyj := struct {
			TableName     string
			IndexName     string
			KeyConditions struct {
				I struct {
					AttributeValueList []struct{ S string }
					ComparisonOperator string
				}
			}
			Limit            int
			ScanIndexForward bool
		}{}
		bodyj.TableName = ddbTablePost
		bodyj.IndexName = "Score"
		bodyj.KeyConditions.I.AttributeValueList = []struct{ S string }{struct{ S string }{S: postType}}
		bodyj.KeyConditions.I.ComparisonOperator = "EQ"
		bodyj.Limit = limit
		bodyj.ScanIndexForward = false
		body, _ = json.Marshal(bodyj)
	} else {
		bodyj := struct {
			TableName     string
			IndexName     string
			KeyConditions struct {
				I struct {
					AttributeValueList []struct{ S string }
					ComparisonOperator string
				}
			}
			ExclusiveStartKey struct {
				I struct{ S string }
				K struct{ B []byte }
				S struct{ N string }
			}
			Limit            int
			ScanIndexForward bool
		}{}
		bodyj.TableName = ddbTablePost
		bodyj.IndexName = "Score"
		bodyj.KeyConditions.I.AttributeValueList = []struct{ S string }{struct{ S string }{S: postType}}
		bodyj.KeyConditions.I.ComparisonOperator = "EQ"
		bodyj.ExclusiveStartKey.I.S = postType
		bodyj.ExclusiveStartKey.K.B = key
		bodyj.ExclusiveStartKey.S.N = fmt.Sprintf("%d", score)
		bodyj.Limit = limit
		bodyj.ScanIndexForward = forward
		body, _ = json.Marshal(bodyj)
	}
	ddbResp := struct {
		Count            int
		LastEvaluatedKey struct {
			I struct{ S string }
			K struct{ B []byte }
		}
		Items []PostDDB
	}{}
	if err := aws.DynamoDBPostBytes("Query", body, &ddbResp); err != nil {
		return nil, err
	}
	return ddbResp.Items, nil
}

func CreateDDBTables() error {
	bodies := []string{
		fmt.Sprintf(`{
  "TableName": "%s",
  "AttributeDefinitions": [
    { "AttributeName": "I", "AttributeType": "S" },
    { "AttributeName": "K", "AttributeType": "B" },
    { "AttributeName": "S", "AttributeType": "N" } ],
  "KeySchema": [
    { "AttributeName": "I", "KeyType": "HASH" },
    { "AttributeName": "K", "KeyType": "RANGE" } ],
  "GlobalSecondaryIndexes":[{
      "IndexName": "Score",
      "KeySchema": [
        { "AttributeName": "I", "KeyType": "HASH" },
        { "AttributeName": "S", "KeyType": "RANGE" } ],
      "Projection": { "ProjectionType": "ALL" },
      "ProvisionedThroughput": {"ReadCapacityUnits":1, "WriteCapacityUnits":1}
  }],
  "ProvisionedThroughput": { "ReadCapacityUnits": 1, "WriteCapacityUnits": 1 }
}`, ddbTablePost),
		fmt.Sprintf(`{
  "TableName": "%s",
  "AttributeDefinitions": [
    { "AttributeName": "D", "AttributeType": "B" },
    { "AttributeName": "P", "AttributeType": "B" } ],
  "KeySchema": [
    { "AttributeName": "D", "KeyType": "HASH" },
    { "AttributeName": "P", "KeyType": "RANGE" } ],
  "ProvisionedThroughput": { "ReadCapacityUnits": 1, "WriteCapacityUnits": 1 }
}`, ddbTableVote),
	}
	for _, b := range bodies {
		if err := aws.DynamoDBPostBytes("CreateTable", []byte(b), nil); err != nil {
			glog.Fatalf("%v", err)
			return err
		}
	}
	return nil
}

type appError struct {
	Message string
	Code    int
}

func (a *appError) Error() string {
	return a.Message
}

func jsonAPI(path string, fn func(w http.ResponseWriter, r *http.Request) *appError) {
	http.DefaultServeMux.HandleFunc(path, makeGzipHandler(makeJSONErrorHandler(fn)))
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func makeGzipHandler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			fn(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		fn(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
	}
}

func makeJSONErrorHandler(fn func(w http.ResponseWriter, r *http.Request) *appError) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		appErr := fn(w, r)
		if appErr != nil {
			je := struct {
				Error string `json:"error"`
			}{
				Error: appErr.Message,
			}
			b, _ := json.Marshal(&je)
			http.Error(w, string(b), appErr.Code)
		}
	}
}

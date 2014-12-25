package hack

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang/glog"

	"hack20141225/aws"
)

const (
	postTypeGIF = "gif"
)

type PostDDB struct {
	I   struct{ S string } // just an index
	K   struct{ B []byte } // a unique key for this post
	S   struct{ N string } // score
	URL struct{ S string } // url of the image
}

func postPK(index string, key []byte) []byte {
	b := append([]byte(index), '\x00')
	b = append(b, key...)
	return b
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
	jsonError("/PostImg", PostImg)
	jsonError("/Hot", Hot)
	jsonError("/Vote", Vote)
}

// PostImg posts an image URL to the server
//   curl http://localhost:8080/PostImg?url=http%3A%2F%2F127.0.0.1%2Fa.jpg
func PostImg(w http.ResponseWriter, r *http.Request) *appError {
	url := r.FormValue("url")

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
		return &appError{Message: err.Error(), Code: http.StatusInternalServerError}
	}
	json.NewEncoder(w).Encode(post)
	return nil
}

// Hot returns the hottest images.
//  curl http://localhost:8080/Hot
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

	posts, err := getPostsByScore(postTypeGIF, key, score, forward, limit)
	if err != nil {
		return &appError{Message: err.Error(), Code: http.StatusInternalServerError}
	}
	json.NewEncoder(w).Encode(posts)
	return nil
}

// Vote votes for an image.
//   curl 'http://localhost:8080/Vote?device_id=ddd&key=E7MySUSwyFQ%3D'
func Vote(w http.ResponseWriter, r *http.Request) *appError {
	deviceID := r.FormValue("device_id")
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
	}{}
	bj.TableName = ddbTablePost
	bj.Key.I.S = postTypeGIF
	bj.Key.K.B = key
	bj.UpdateExpression = "ADD S :s"
	bj.ExpressionAttributeValues.S.N = "1"
	if err := aws.DynamoDBPost("UpdateItem", bj, nil); err != nil {
		glog.Errorf("%v", err)
	}

	w.Write([]byte("true"))
	return nil
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

func jsonError(path string, fn func(w http.ResponseWriter, r *http.Request) *appError) {
	jsonErrorServeMux(http.DefaultServeMux, path, fn)
}

func jsonErrorServeMux(sm *http.ServeMux, path string, fn func(w http.ResponseWriter, r *http.Request) *appError) {
	sm.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
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
	})
}

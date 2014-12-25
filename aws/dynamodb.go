package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/smartystreets/go-aws-auth"
)

type ErrDynamoDB struct {
	Type    string `json:"__type"`
	Message string
}

func (e *ErrDynamoDB) Error() string {
	return e.Message
}

var dynamoDBEndpoint *url.URL

func DynamoDBPost(operation string, reqb interface{}, respj interface{}) error {
	body, err := json.Marshal(reqb)
	if err != nil {
		return err
	}
	return DynamoDBPostBytes(operation, body, respj)
}

func DynamoDBPostBytes(operation string, body []byte, respj interface{}) error {
	req, err := http.NewRequest("POST", dynamoDBEndpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("x-amz-target", "DynamoDB_20120810."+operation)
	req.Header.Set("host", dynamoDBEndpoint.Host)
	req.Header.Set("content-type", "application/x-amz-json-1.0")
	awsauth.Sign4(req, Credentials())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	//fmt.Printf("DynamoDBPostBytes operation: %s body: %s, statusCode: %d, resp: %s\n", operation, body, resp.StatusCode, respBody)

	if resp.StatusCode != 200 {
		derr := &ErrDynamoDB{}
		if err := json.Unmarshal(respBody, derr); err != nil {
			return fmt.Errorf(string(respBody))
		}
		z := strings.SplitN(derr.Type, "#", 2)
		if len(z) != 2 {
			return fmt.Errorf(string(respBody))
		}
		derr.Type = z[1]
		return derr
	}

	if respj == nil {
		return nil
	}
	err = json.Unmarshal(respBody, respj)
	if err != nil {
		return fmt.Errorf(`json error: %v, data: %s`, err, string(respBody))
	}
	return nil
}

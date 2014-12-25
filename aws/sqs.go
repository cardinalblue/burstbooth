package aws

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/smartystreets/go-aws-auth"
)

var SQSEndpoint string

func SQSPost(queueURL string, values url.Values, resp interface{}) error {
	req, err := http.NewRequest("POST", queueURL, bytes.NewReader([]byte(values.Encode())))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	awsauth.Sign4(req, Credentials())
	httpresp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer httpresp.Body.Close()
	respbody, err := ioutil.ReadAll(httpresp.Body)
	if err != nil {
		return err
	}

	if httpresp.StatusCode != 200 {
		eresp := &ErrorResponse{}
		if err := xml.Unmarshal(respbody, eresp); err != nil {
			return fmt.Errorf("%s", respbody)
		}
		return eresp
	}

	if resp == nil {
		return nil
	}
	if err := xml.Unmarshal(respbody, resp); err != nil {
		return fmt.Errorf("xml error: %v, data: %s", err, respbody)
	}
	return nil
}

func GetQueueURL(queueName, accountID string) (string, error) {
	v := url.Values{"Action": {"GetQueueUrl"}, "QueueName": {queueName}}
	if accountID != "" {
		v.Set("QueueOwnerAWSAccountId", accountID)
	}
	res := GetQueueURLResult{}
	if err := SQSPost(SQSEndpoint, v, &res); err != nil {
		return "", err
	}
	return res.QueueURL, nil
}

func ListQueues(prefix string) ([]string, error) {
	v := url.Values{"Action": {"ListQueues"}}
	if prefix != "" {
		v.Set("QueueNamePrefix", prefix)
	}
	res := ListQueuesResult{}
	if err := SQSPost(SQSEndpoint, v, &res); err != nil {
		return nil, err
	}
	return res.QueueURLs, nil
}

func ChangeMessageVisibility(queueURL, receiptHandle string, visibilityTimeout int) error {
	v := url.Values{
		"Action":            {"ChangeMessageVisibility"},
		"ReceiptHandle":     {receiptHandle},
		"VisibilityTimeout": {fmt.Sprintf("%d", visibilityTimeout)},
	}
	if err := SQSPost(queueURL, v, nil); err != nil {
		return err
	}
	return nil
}

func DeleteMessage(queueURL, receiptHandle string) error {
	v := url.Values{
		"Action":        {"DeleteMessage"},
		"ReceiptHandle": {receiptHandle},
	}
	if err := SQSPost(queueURL, v, nil); err != nil {
		return err
	}
	return nil
}

func CreateQueue(name string, v url.Values) (string, error) {
	if v == nil {
		v = url.Values{}
	}
	v.Set("Action", "CreateQueue")
	v.Set("QueueName", name)
	res := struct {
		QueueURL string `xml:"CreateQueueResult>QueueUrl"`
	}{}
	if err := SQSPost(SQSEndpoint, v, &res); err != nil {
		return "", err
	}
	return res.QueueURL, nil
}

func DeleteQueue(queueURL string) error {
	v := url.Values{"Action": {"DeleteQueue"}}
	if err := SQSPost(queueURL, v, nil); err != nil {
		return err
	}
	return nil
}

type ErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Type      string   `xml:"Error>Type"`
	Code      string   `xml:"Error>Code"`
	Message   string   `xml:"Error>Message"`
	RequestID string   `xml:"RequestId"`
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type ListQueuesResult struct {
	QueueURLs []string `xml:"ListQueuesResult>QueueUrl"`
}

type CreateQueueResult struct {
	QueueURL string `xml:"CreateQueueResult>QueueUrl"`
}

type GetQueueURLResult struct {
	QueueURL string `xml:"GetQueueUrlResult>QueueUrl"`
}

type SendMessageResult struct {
	MD5OfMessageAttributes string `xml:"SendMessageResult>MD5OfMessageAttributes"`
	MD5OfMessageBody       string `xml:"SendMessageResult>MD5OfMessageBody"`
	MessageID              string `xml:"SendMessageResult>MessageId"`
}

type ReceiveMessageResult struct {
	Messages []Message `xml:"ReceiveMessageResult>Message"`
}

type Message struct {
	Attributes             map[string]string                `xml:"Attribute"`
	Body                   string                           `xml:"Body"`
	MD5OfBody              string                           `xml:"MD5OfBody"`
	MD5OfMessageAttributes string                           `xml:"MD5OfMessageAttributes"`
	MessageAttributes      map[string]MessageAttributeValue `xml:"MessageAttribute"`
	MessageID              string                           `xml:"MessageId"`
	ReceiptHandle          string                           `xml:"ReceiptHandle"`
}

type MessageAttributeValue struct {
	BinaryListValues [][]byte `xml:"BinaryListValue>BinaryListValue"`
	BinaryValue      []byte   `xml:"BinaryValue"`
	DataType         string   `xml:"DataType"`
	StringListValues []string `xml:"StringListValue>StringListValue"`
	StringValue      string   `xml:"StringValue"`
}

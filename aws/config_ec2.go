// +build ec2

package aws

import (
	"net/url"

	"github.com/golang/glog"
)

func init() {
	region, err := Region()
	if err != nil {
		glog.Fatalf("%v", err)
	}
	dynamoDBEndpoint, err = url.Parse("http://dynamodb." + region + ".amazonaws.com/")
	if err != nil {
		glog.Fatalf("%v", err)
	}
	SQSEndpoint = "http://sqs." + region + ".amazonaws.com/"
}

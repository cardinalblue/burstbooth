// +build local

package aws

import (
	"net/url"
	"os"

	"github.com/golang/glog"
)

func init() {
	var err error
	dynamoDBEndpoint, err = url.Parse("http://localhost:8000")
	if err != nil {
		glog.Fatalf("%v", err)
	}
	SQSEndpoint = "http://localhost:" + os.Getenv("SQS_PORT")
}

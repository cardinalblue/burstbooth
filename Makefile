DDB_TABLES=DDB_TABLE_POST=Post
DDB_TABLES+= DDB_TABLE_VOTE=Vote

ec2:
	git archive --output=ec2.zip HEAD
	zip -r ec2.zip ./Dockerfile

local:
	rm -f -r ${GOPATH}/pkg/darwin_amd64/github.com/cardinalblue/burstbooth
	go get -tags local github.com/cardinalblue/burstbooth/bin/server
	AWS_ACCESS_KEY_ID=BurstboothDev ${DDB_TABLES} ${GOPATH}/bin/server -logtostderr=true -stderrthreshold=INFO
	# ln -f -s ./Dockerfile.local ./Dockerfile
	# docker build -t maps-local .
	# docker run -e AWS_ACCESS_KEY_ID=mapsdev -p 8080:8080 maps-local
	# open http://192.168.59.103:8080/

test:
	AWS_ACCESS_KEY_ID=BurstboothTest ${DDB_TABLES} go test -tags local . -logtostderr=true -stderrthreshold=INFO

localddb:
	AWS_ACCESS_KEY_ID=BurstboothDev ${DDB_TABLES} go run -tags local bin/setupddb/main.go

clean:
	rm -f ec2.zip

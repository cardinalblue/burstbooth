DDB_TABLES=DDB_TABLE_POST=Post
DDB_TABLES+= DDB_TABLE_VOTE=Vote

local:
	rm -f -r ${GOPATH}/pkg/darwin_amd64/hack20141225
	go get -tags local hack20141225/bin/server
	AWS_ACCESS_KEY_ID=hack20141225Dev ${DDB_TABLES} ${GOPATH}/bin/server -logtostderr=true -stderrthreshold=INFO
	# ln -f -s ./Dockerfile.local ./Dockerfile
	# docker build -t maps-local .
	# docker run -e AWS_ACCESS_KEY_ID=mapsdev -p 8080:8080 maps-local
	# open http://192.168.59.103:8080/

test:
	AWS_ACCESS_KEY_ID=hack20141225Test ${DDB_TABLES} go test -tags local . -logtostderr=true -stderrthreshold=INFO

localddb:
	AWS_ACCESS_KEY_ID=hack20141225Dev ${DDB_TABLES} go run -tags local bin/setupddb/main.go

FROM golang

ENV package github.com/cardinalblue/burstbooth
WORKDIR /go/src/${package}
ADD . /go/src/${package}
RUN go get -tags ec2 ${package}/bin/server

EXPOSE 8080 8000
CMD ["-log_dir=/var/log/burstbooth", "-stderrthreshold=4"]
ENTRYPOINT ["/go/bin/server"]

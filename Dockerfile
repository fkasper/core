#FROM phusion/baseimage
FROM golang:1.5-onbuild
EXPOSE 8111
RUN make install

ENTRYPOINT ["/go/bin/core"]

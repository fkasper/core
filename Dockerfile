#FROM phusion/baseimage
FROM golang:1.5-onbuild
EXPOSE 8111
RUN make install

#COPY templates .

CMD ["/go/bin/core"]

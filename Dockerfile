FROM golang:1.4-onbuild
EXPOSE 8111
RUN make install
ENTRYPOINT ["/go/bin/xcms-core"]

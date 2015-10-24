
test: clean
	go test -v ./... -cover

clean:
	find . -name flymake_* -delete

update:
	rm -rf Godeps/
	find . -iregex .*go | xargs sed -i 's:".*Godeps/_workspace/src/:":g'
	godep save -r ./...

cover-package: clean
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

sloccount:
	 find . -path ./Godeps -prune -o -name "*.go" -print0 | xargs -0 wc -l

install: clean
	go install github.com/fkasper/core
	cd xctl && $(MAKE) install && cd ..
	#cd vbundle && $(MAKE) install && cd ..

run: install
	xcms-core -etcd=${ETCD_NODE1} -etcd=${ETCD_NODE2} -etcd=${ETCD_NODE3} -etcdKey=/xcms -statsdAddr=localhost:8125 -statsdPrefix=xcms -logSeverity=INFO

run-fast: install
	xcms-core -etcd=${ETCD_NODE1} -etcd=${ETCD_NODE2} -etcd=${ETCD_NODE3} -etcdKey=/xcms

docker-clean:
	docker rm -f xcms-core

build:
	go build -a -tags netgo -installsuffix cgo -ldflags '-w' -o ./xcms-core .
# docker-build:
# 	go build -a -tags netgo -installsuffix cgo -ldflags '-w' -o ./xcms-core .
# 	go build -a -tags netgo -installsuffix cgo -ldflags '-w' -o ./vctl/vctl ./vctl
# 	go build -a -tags netgo -installsuffix cgo -ldflags '-w' -o ./vbundle/vbundle ./vbundle
# 	docker build -t mailgun/xcms-core:latest -f ./Dockerfile-scratch .

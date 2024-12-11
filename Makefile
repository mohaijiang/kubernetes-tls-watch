build:
	GOPROXY=https://goproxy.cn go mod tidy
	go build

build-image:
	docker buildx build -t mohaijiang/kubernetes-tls-watch . --push

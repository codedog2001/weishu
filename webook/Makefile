.PHONY: docker
docker:
	@rm webook ||  true
	@go mod tidy
#为 ARM 架构的 Linux 系统编译当前目录下的 Go 项目，输出文件名为 webook，并使用 -tags=k8s 指定编译标签。
	@GOOS=linux GOARCH=arm go build -tags=k8s -o webook .
	@docker rmi -f zx/webook-live:v0.0.1
#下面这个命令表示使用当前目录下的 Dockerfile 来构建一个新的 Docker 镜像，并标记为 ZX/webook:v0.0.1。
	@docker build -t zx/webook-live:v0.0.1 .
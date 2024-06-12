#基础镜像
FROM ubuntu:20.04
#把编译后的打包进来这个镜像生成webook，放到工作目录/app,这个目录任意指定
COPY webook /app/webook
WORKDIR /app
#CMD是执行单条命令
#最佳
ENTRYPOINT ["/app/webook"]
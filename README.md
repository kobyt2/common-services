# golang 的common-services里面包含的一些基础功能

日志是基于 Zap 的简单日志库，支持根据日志级别记录到不同的文件 可根据yaml文件配置调整。
加密是采用 bcrypt、以及AES的ecb,cbc模式、RSA模式

## Installation
go get github.com/kobyt2/common-services/logger

# go-zap-logger
可以根据配置文件自定义日志格式等，如不创建配置文件默认是json格式日志输出

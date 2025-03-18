# shimmerdata
Golang SDK for Shimmer BI System

本SDK兼容数数科技的数据格式，可以从数数科技无缝切换到本系统。

## 1.如何使用该SDK
```
go get github.com/ShimmerGames-Co-Ltd/shimmerdata-go
```
## 2.日志格式
shimmerdata只是json格式的日志。
在SDK中以map的方式传递数据。
shimmerdata兼容数数科技的数据格式，可以从数数SDK直接切换过来，不需要对日志格式做任何修改。
## 3.日志传输方式
shimmerdata使用HTTP传输日志，因此需要先到日志收集服注册APP，注册完后会获得一个APPID和APPTOKEN，这两个参数是日志上报的必须参数。
为了保证日志的完整性，shimmerdata支持了日志缓存，当HTTP服务不可用时日志会被保存到临时文件夹，服务恢复后以文件的形式上传到服务器。
## 4.代码示例
请查看examples目录中的代码示例。

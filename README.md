# Monibuca 的RTSP 插件

主要功能是提供RTSP的端口监听接受RTSP推流，以及对RTSP地址进行拉流转发

## 插件名称

RTSP

## 配置
```toml
[RTSP]
ListenAddr  = ":554"
BufferLength  = 2048
AutoPull     = false
RemoteAddr   = "rtsp://localhost/${streamPath}"
[[RTSP.AutoPullList]]
URL = "rtsp://admin:admin@192.168.1.212:554/cam/realmonitor?channel=1&subtype=1"
StreamPath = "live/rtsp"
```
- ListenAddr 是监听端口，可以将rtsp流推到Monibuca中
- BufferLength是指解析拉取的rtp包的缓冲大小
- AutoPull是指当有用户订阅一个新流的时候自动向远程拉流转发
- RemoteAddr 指远程拉流地址，其中${streamPath}是占位符，实际使用流路径替换。
- AutoPullList 是一个数组，如果配置了该数组，则会在程序启动时自动启动拉流，StreamPath一定要是唯一的，不能重复

## 使用方法(拉流转发)
```go
new(RTSP).PullStream("live/user1","rtsp://xxx.xxx.xxx.xxx/live/user1") 
```
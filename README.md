# Monibuca 的RTSP 插件

主要功能是对RTSP地址进行拉流转换

## 插件名称

RTSP

## 配置
```toml
[RTSP]
BufferLength  = 2048
AutoPull     = false
RemoteAddr   = "rtsp://localhost/${streamPath}"
```
- BufferLength是指解析拉取的rtp包的缓冲大小
- AutoPull是指当有用户订阅一个新房间的时候自动向远程拉流转发
- RemoteAddr 指远程拉流地址，其中${streamPath}是占位符，实际使用流路径替换。


## 使用方法(拉流转发)
```go
new(RTSP).Publish("live/user1","rtsp://xxx.xxx.xxx.xxx/live/user1") 
```
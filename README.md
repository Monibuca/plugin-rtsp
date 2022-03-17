# RTSP插件

## 插件地址

github.com/Monibuca/plugin-rtsp

## 插件引入
```go
import (
    _ "m7s.live/plugin/rtsp/v4"
)
```

## 默认插件配置

```yaml
rtsp:
# 端口接收推流
  listenaddr: :554
  udpaddr: :8000
  readbuffersize: 2048
```
### 特殊功能

当自动拉流列表中当的streamPath为sub/xxx 这种形式的话，在gb28181的分屏显示时会优先采用rtsp流，已实现分屏观看子码流效果
## 插件功能

### 接收RTSP协议的推流

例如通过ffmpeg向m7s进行推流

```bash
ffmpeg -i **** rtsp://localhost/live/test
```

会在m7s内部形成一个名为live/test的流

### 从远程拉取rtsp到m7s中

可调用接口
`rtsp/api/pull?target=[RTSP地址]&streamPath=[流标识]`

## 使用编程方式拉流
```go
new(RTSPClient).PullStream("live/user1","rtsp://xxx.xxx.xxx.xxx/live/user1") 
```

### 罗列所有的rtsp协议的流

可调用接口
`rtsp/api/list`

### 从m7s中拉取rtsp协议流

直接通过协议rtsp://xxx.xxx.xxx.xxx/live/user1 即可播放

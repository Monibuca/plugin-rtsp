# RTSP插件
rtsp插件提供rtsp协议的推拉流能力，以及向远程服务器推拉rtsp协议的能力。
## 插件地址

https://github.com/Monibuca/plugin-rtsp

## 插件引入
```go
import (
    _ "m7s.live/plugin/rtsp/v4"
)
```

## 推拉地址形式
```
rtsp://localhost/live/test
```
- `localhost`是m7s的服务器域名或者IP地址，默认端口`554`可以不写，否则需要写
- `live`代表`appName`
- `test`代表`streamName`
- m7s中`live/test`将作为`streamPath`为流的唯一标识


例如通过ffmpeg向m7s进行推流

```bash
ffmpeg -i [视频源] -c:v h264 -f rtsp rtsp://localhost/live/test
```

会在m7s内部形成一个名为live/test的流


如果m7s中已经存在live/test流的话就可以用rtsp协议进行播放
```bash
ffplay rtsp://localhost/live/test
```


## 配置

```yaml
rtsp:
    publish:
        pubaudio: true
        pubvideo: true
        kickexist: false
        publishtimeout: 10
        waitclosetimeout: 0
    subscribe:
        subaudio: true
        subvideo: true
        iframeonly: false
        waittimeout: 10
    pull:
        repull: 0
        pullonstart: false
        pullonsubscribe: false
        pulllist: {}
    push:
        repush: 0
        pushlist: {}
    listenaddr: :554
    udpaddr: :8000
    rtcpaddr: :8001
    readbuffersize: 2048
    pullprotocol: 'auto'
```
:::tip 配置覆盖
publish
subscribe
两项中未配置部分将使用全局配置
:::
## API

### `rtsp/api/list`
获取所有rtsp流

### `rtsp/api/pull?target=[RTSP地址]&streamPath=[流标识]`
从远程拉取rtsp到m7s中

### `rtsp/api/push?target=[RTSP地址]&streamPath=[流标识]`
将本地的流推送到远端
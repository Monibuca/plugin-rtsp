_[English](https://github.com/Monibuca/plugin-rtsp/blob/v4/README.en.md) | 简体中文_
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
ffmpeg -i [视频源] -c:v h264 -c:a aac -f rtsp rtsp://localhost/live/test
```

会在m7s内部形成一个名为live/test的流


如果m7s中已经存在live/test流的话就可以用rtsp协议进行播放
```bash
ffplay rtsp://localhost/live/test
```

## 配置

```yaml
rtsp:
    publish: # 参考全局配置格式
    subscribe: # 参考全局配置格式
    pull: # 格式参考文档 https://m7s.live/guide/config.html#%E6%8F%92%E4%BB%B6%E9%85%8D%E7%BD%AE
    push: # 格式参考文档 https://m7s.live/guide/config.html#%E6%8F%92%E4%BB%B6%E9%85%8D%E7%BD%AE
    listenaddr: :554
    udpaddr: :8000
    rtcpaddr: :8001
    readbuffercount: 2048 # 读取缓存队列大小
    writebuffercount: 2048 # 写出缓存队列大小
    pullprotocol: tcp # auto, tcp, udp
```
:::tip 配置覆盖
publish
subscribe
两项中未配置部分将使用全局配置
:::
## API

### `rtsp/api/list`
获取所有rtsp流

### `rtsp/api/pull?target=[RTSP地址]&streamPath=[流标识]&save=[0|1|2]`
从远程拉取rtsp到m7s中
- save含义：0、不保存；1、保存到pullonstart；2、保存到pullonsub
- RTSP地址需要进行urlencode 防止其中的特殊字符影响解析
### `rtsp/api/push?target=[RTSP地址]&streamPath=[流标识]`
将本地的流推送到远端
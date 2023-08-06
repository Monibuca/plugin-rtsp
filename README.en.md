_[简体中文](https://github.com/Monibuca/plugin-rtsp) | English_
# RTSP Plugin

The RTSP plugin provides the ability to push and pull the RTSP protocol and also to push and pull the RTSP protocol to remote servers.

## Plugin address

https://github.com/Monibuca/plugin-rtsp

## Plugin introduction

```go
import (
    _ "m7s.live/plugin/rtsp/v4"
)
```

## Push and Pull address form

```
rtsp://localhost/live/test
```
- `localhost` is the m7s server domain name or IP address, and the default port `554` can be omitted, otherwise it is required to be written.
- `live` represents `appName`
- `test` represents `streamName`
- `live/test` in m7s will serve as the stream identity.

For example, push stream to m7s through ffmpeg

```bash
ffmpeg -i [video source] -c:v h264 -c:a aac -f rtsp rtsp://localhost/live/test
```

This will create a stream named `live/test` inside m7s.

If the `live/test` stream already exists in m7s, then you can use the RTSP protocol to play it.

```bash
ffplay rtsp://localhost/live/test
```

## Configuration

```yaml
rtsp:
    publish: # Refer to the global configuration format
    subscribe: # Refer to the global configuration format
    pull: # Format reference document https://m7s.live/guide/config.html#%E6%8F%92%E4%BB%B6%E9%85%8D%E7%BD%AE
    push: # Format reference document https://m7s.live/guide/config.html#%E6%8F%92%E4%BB%B6%E9%85%8D%E7%BD%AE
    listenaddr: :554
    udpaddr: :8000
    rtcpaddr: :8001
    readbuffercount: 2048
    writebuffercount: 2048
    pullprotocol: 'auto' # auto, tcp, udp
```
:::tip Configuration override
publish and subscribe, any section not configured will use global configuration.
:::

## API

### `rtsp/api/list`
Get all RTSP streams

### `rtsp/api/pull?target=[RTSP address]&streamPath=[Stream identity]&save=[0|1|2]`
Pull the RTSP to m7s from a remote server
save meaning: 0, do not save; 1, save to pullonstart; 2, save to pullonsub
### `rtsp/api/push?target=[RTSP address]&streamPath=[Stream identity]`
Push local streams to remote servers
<template>
    <div>
        <Button @click="addPull" type="success">拉流转发</Button>
        <Spin fix v-if="Rooms==null">
            <Icon type="ios-loading" size="18" class="demo-spin-icon-load"></Icon>
            <div>Loading</div>
        </Spin>
        <div v-else-if="Rooms.length==0" class="empty">
            <Icon type="md-wine" size="50" />没有任何房间
        </div>
        <div class="layout" v-else>
            <Card v-for="item in Rooms" :key="item.RoomInfo.StreamPath" class="room">
                <p slot="title">{{item.RoomInfo.StreamPath}}</p>
                <StartTime slot="extra" :value="item.RoomInfo.StartTime"></StartTime>
                <div class="hls-info">
                    <Progress :stroke-width="20" :percent="Math.ceil(item.BufferRate)" text-inside />
                    <div>📜{{item.SyncCount}}</div>
                </div>
                <Button @click="showHeader(item)">
                    <Icon type="ios-code-working" />
                </Button>
            </Card>
        </div>
    </div>
</template>

<script>
let listES = null;
import StartTime from "./components/StartTime";
export default {
    components: {
        StartTime
    },
    data() {
        return {
            currentStream: null,
            Rooms: null,
            remoteAddr: "",
            streamPath: ""
        };
    },

    methods: {
        fetchlist() {
            listES = new EventSource("/rtsp/list");
            listES.onmessage = evt => {
                if (!evt.data) return;
                this.Rooms = JSON.parse(evt.data) || [];
                this.Rooms.sort((a, b) =>
                    a.RoomInfo.StreamPath > b.RoomInfo.StreamPath ? 1 : -1
                );
            };
        },
        showHeader(item) {
            this.$Modal.info({
                title: "RTSP Header",
                width: "1000px",
                scrollable: true,
                content: item.Header
            });
        },
        addPull() {
            this.$Modal.confirm({
                title: "拉流转发",
                onOk:()=> {
                    window.ajax
                        .getJSON("/rtsp/pull", {
                            target: this.remoteAddr,
                            streamPath: this.streamPath
                        })
                        .then(x => {
                            if (x.code == 0) {
                                this.$Message.success({
                                    title: "提示",
                                    content: "已启动拉流"
                                });
                            } else {
                                this.$Message.error({
                                    title: "提示",
                                    content: x.msg
                                });
                            }
                        });
                },
                render: h => {
                    return h("div", {}, [
                        h("Input", {
                            props: {
                                value: this.remoteAddr,
                                autofocus: true,
                                placeholder: "Please enter URL of rtsp..."
                            },
                            on: {
                                input: val => {
                                    this.remoteAddr = val;
                                }
                            }
                        }),
                        h("Input", {
                            props: {
                                value: this.streamPath,
                                placeholder:
                                    "Please enter streamPath to publish."
                            },
                            on: {
                                input: val => {
                                    this.streamPath = val;
                                }
                            }
                        })
                    ]);
                }
            });
        }
    },
    mounted() {
        this.fetchlist();
    },
    deactivated() {
        listES.close();
    }
};
</script>

<style>
@import url("/iview.css");
.empty {
    color: #eb5e46;
    width: 100%;
    min-height: 500px;
    display: flex;
    justify-content: center;
    align-items: center;
}

.layout {
    padding-bottom: 30px;
    display: flex;
    flex-wrap: wrap;
}
.ts-info {
    width: 300px;
}

.hls-info {
    width: 350px;
    display: flex;
    flex-direction: column;
}
</style>
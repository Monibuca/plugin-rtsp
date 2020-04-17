<template>
    <div v-loading="Rooms==null">
        <div v-if="Rooms==null"></div>
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
        <mu-dialog title="拉流转发" width="360" :open.sync="openPull">
            <mu-text-field v-model="remoteAddr" label="rtsp url" label-float help-text="Please enter URL of rtsp...">
            </mu-text-field>
            <mu-text-field v-model="streamPath" label="streamPath" label-float
                help-text="Please enter streamPath to publish."></mu-text-field>
            <mu-button slot="actions" flat color="primary" @click="addPull">确定</mu-button>
        </mu-dialog>
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
            streamPath: "",
            openPull: false
        };
    },

    methods: {
        fetchlist() {
            listES = new EventSource(this.apiHost + "/rtsp/list");
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
            this.openPull = false;
            this.ajax
                .getJSON(this.apiHost + "/rtsp/pull", {
                    target: this.remoteAddr,
                    streamPath: this.streamPath
                })
                .then(x => {
                    if (x.code == 0) {
                        this.$toast.success("已启动拉流");
                    } else {
                        this.$toast.error(x.msg);
                    }
                });
        }
    },
    mounted() {
        this.fetchlist();
        this.$parent.menus = [
            {
                label: "拉流转发",
                action: () => {
                    this.openPull = true;
                }
            }
        ];
    },
    destroyed() {
        listES.close();
        this.$parent.menus = [];
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
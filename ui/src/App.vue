<template>
  <div>
    <mu-data-table :data="Streams" :columns="columns">
      <template #default="{row:item}">
        <td>{{item.StreamInfo.StreamPath}}</td>
        <td>
          <StartTime :value="item.StreamInfo.StartTime"></StartTime>
        </td>
        <td>{{item.InBytes}}</td>
        <td>{{item.OutBytes}}</td>
        <td>
          <mu-button flat @click="showHeader(item)">头信息</mu-button>
          <mu-button flat @click="stop(item)">中止</mu-button>
        </td>
      </template>
    </mu-data-table>
    <mu-dialog title="拉流转发" width="360" :open.sync="openPull">
      <mu-text-field
        v-model="remoteAddr"
        label="rtsp url"
        label-float
        help-text="Please enter URL of rtsp..."
      ></mu-text-field>
      <mu-text-field
        v-model="streamPath"
        label="streamPath"
        label-float
        help-text="Please enter streamPath to publish."
      ></mu-text-field>
      <mu-button slot="actions" flat color="primary" @click="addPull">确定</mu-button>
    </mu-dialog>
  </div>
</template>

<script>
let listES = null;
export default {
  data() {
    return {
      currentStream: null,
      Streams: null,
      remoteAddr: "",
      streamPath: "",
      openPull: false,
      columns: [
        "StreamPath",
        "开始时间",
        "总接收",
        "总发送",
        "操作"
      ].map(title => ({ title }))
    };
  },

  methods: {
    fetchlist() {
      listES = new EventSource(this.apiHost + "/rtsp/list");
      listES.onmessage = evt => {
        if (!evt.data) return;
        this.Streams = JSON.parse(evt.data) || [];
        this.Streams.sort((a, b) =>
          a.StreamInfo.StreamPath > b.StreamInfo.StreamPath ? 1 : -1
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
    },
    stop(item) {
      this.ajax
        .get(this.apiHost + "/api/stop", {
          stream: item.StreamInfo.StreamPath
        })
        .then(x => {
          if (x == "success") {
            this.$toast.success("已停止拉流");
          } else {
            this.$toast.error(x.msg);
          }
        });
    }
  },
  mounted() {
    this.fetchlist();
    let _this = this;
    this.$parent.titleOps = [
      {
        template: '<m-button @click="onClick">拉流转发</m-button>',
        methods: {
          onClick() {
            _this.openPull = true;
          }
        }
      }
    ];
  },
  destroyed() {
    listES.close();
  }
};
</script>

<style>
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
<template>
  <div class="player-wrapper">
    <!-- 状态栏 -->
    <div class="status-bar">
      <span class="status-dot" :class="statusClass"></span>
      <span class="status-text">{{ statusText }}</span>
    </div>

    <!-- 视频元素：muted + autoplay + playsinline（iOS 需要） -->
    <video
      ref="videoEl"
      autoplay
      playsinline
      muted
      class="video"
    ></video>

    <!-- 底部控制栏 -->
    <div class="controls">
      <button v-if="isMuted" class="btn-unmute" @click="unmute">
        🔇 开启声音
      </button>
      <button v-else class="btn-unmute active" @click="mute">
        🔊 已开启声音
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'

// ICE 候选收集超时时间（毫秒）
const ICE_GATHER_TIMEOUT_MS = 3000
// 断线自动重连延迟（毫秒）
const RECONNECT_DELAY_MS = 5000

// 连接状态：connecting | live | disconnected
const status = ref('connecting')
const isMuted = ref(true)
const videoEl = ref(null)

let pc = null           // RTCPeerConnection 实例
let reconnectTimer = null  // 断线重连定时器

// 状态显示文字
const statusText = computed(() => {
  const map = {
    connecting: '连接中...',
    live: '🔴 直播中',
    disconnected: '已断线，5秒后重连...',
  }
  return map[status.value] || status.value
})

// 状态点颜色样式
const statusClass = computed(() => ({
  'dot-connecting': status.value === 'connecting',
  'dot-live': status.value === 'live',
  'dot-disconnected': status.value === 'disconnected',
}))

// 开启声音
function unmute() {
  if (videoEl.value) {
    videoEl.value.muted = false
    isMuted.value = false
  }
}

// 静音
function mute() {
  if (videoEl.value) {
    videoEl.value.muted = true
    isMuted.value = true
  }
}

// 建立 WHEP 拉流连接
async function connect() {
  // 清理旧连接
  if (pc) {
    pc.close()
    pc = null
  }

  status.value = 'connecting'

  try {
    pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.miwifi.com:3478' }],
    })

    // 只接收视频和音频
    pc.addTransceiver('video', { direction: 'recvonly' })
    pc.addTransceiver('audio', { direction: 'recvonly' })

    // 收到媒体流时绑定到 <video>
    pc.ontrack = (event) => {
      if (videoEl.value && event.streams && event.streams[0]) {
        videoEl.value.srcObject = event.streams[0]
        status.value = 'live'
      }
    }

    // 连接状态变化监听
    pc.oniceconnectionstatechange = () => {
      const s = pc?.iceConnectionState
      if (s === 'failed' || s === 'disconnected' || s === 'closed') {
        status.value = 'disconnected'
        scheduleReconnect()
      } else if (s === 'connected' || s === 'completed') {
        status.value = 'live'
        clearReconnectTimer()
      }
    }

    // 创建 offer
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)

    // 等待 ICE 候选收集完成（兼容旧版 Safari）
    await waitForICEGathering(pc)

    // 发送 offer 到后端
    const response = await fetch('/whep', {
      method: 'POST',
      headers: { 'Content-Type': 'application/sdp' },
      body: pc.localDescription.sdp,
    })

    if (!response.ok) {
      throw new Error(`WHEP 请求失败: ${response.status}`)
    }

    const answerSDP = await response.text()
    await pc.setRemoteDescription({
      type: 'answer',
      sdp: answerSDP,
    })
  } catch (err) {
    console.error('连接错误:', err)
    status.value = 'disconnected'
    scheduleReconnect()
  }
}

// 等待 ICE 候选收集完成（最多等待 3 秒）
function waitForICEGathering(peerConnection) {
  return new Promise((resolve) => {
    if (peerConnection.iceGatheringState === 'complete') {
      resolve()
      return
    }
    const timeout = setTimeout(resolve, ICE_GATHER_TIMEOUT_MS)
    peerConnection.onicegatheringstatechange = () => {
      if (peerConnection.iceGatheringState === 'complete') {
        clearTimeout(timeout)
        resolve()
      }
    }
  })
}

// 计划断线重连（5 秒后）
function scheduleReconnect() {
  clearReconnectTimer()
  reconnectTimer = setTimeout(() => {
    connect()
  }, RECONNECT_DELAY_MS)
}

// 取消重连定时器
function clearReconnectTimer() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

onMounted(() => {
  connect()
})

onUnmounted(() => {
  clearReconnectTimer()
  if (pc) {
    pc.close()
    pc = null
  }
})
</script>

<style scoped>
.player-wrapper {
  width: 100%;
  max-width: 1280px;
  background: #111;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.6);
}

.status-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: #1a1a1a;
  font-size: 0.85rem;
  color: #aaa;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-connecting {
  background: #f59e0b;
  animation: pulse 1.2s infinite;
}

.dot-live {
  background: #10b981;
}

.dot-disconnected {
  background: #ef4444;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.video {
  width: 100%;
  display: block;
  background: #000;
  aspect-ratio: 16 / 9;
  object-fit: contain;
}

.controls {
  padding: 12px 16px;
  background: #1a1a1a;
  display: flex;
  gap: 12px;
}

.btn-unmute {
  padding: 8px 16px;
  border: 1px solid #333;
  border-radius: 6px;
  background: #222;
  color: #fff;
  cursor: pointer;
  font-size: 0.9rem;
  transition: background 0.2s;
}

.btn-unmute:hover {
  background: #333;
}

.btn-unmute.active {
  border-color: #10b981;
  color: #10b981;
}
</style>

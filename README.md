# 实时直播 - RDP Stream

OBS 通过 WHIP 推流 → Go SFU → 浏览器 WebRTC 播放

## 启动方法

### 后端
```bash
go run main.go
# 服务监听 :8080
```

### 前端（开发模式）
```bash
cd frontend
npm install
npm run dev
# 访问 http://localhost:5173
```

### 前端（生产模式）
```bash
cd frontend && npm run build
go run main.go
# 访问 http://localhost:8080
```

## OBS 配置

1. 设置 → 推流 → 服务：**WHIP**
2. 服务器地址：`http://localhost:8080/whip`
3. 视频编码器：**H.264**（Baseline Profile）
4. 码率：5000–10000 Kbps
5. 音频编码器：**AAC** 或 **Opus**

## 技术栈

- 后端：Go + pion/webrtc（SFU 转发）
- 前端：Vue 3 + Vite（原生 WebRTC）
- 推流：OBS WHIP 输出

## 说明

- 局域网使用，STUN 使用小米 `stun.miwifi.com:3478`（国内可用）
- 支持 Chrome 92+ 和 iOS 12 Safari（H.264 + PCMA/PCMU fallback）
- 同时只支持一个推流端（单主播）

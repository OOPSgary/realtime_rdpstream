// Package sfu 实现简单的 SFU（选择性转发单元）
// 将 publisher（OBS 推流端）的 RTP 包转发给所有 subscriber（浏览器观看端）
package sfu

import (
	"io"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v3"
)

// streamID 是 SFU 中所有本地 Track 共用的 stream 标识
const streamID = "realtime-stream"

// SFU 管理一个 publisher 和多个 subscriber
type SFU struct {
	mu          sync.RWMutex
	publisher   *webrtc.PeerConnection
	subscribers map[string]*webrtc.PeerConnection
	// 本地 Track，用于向所有 subscriber 转发音视频
	videoTrack *webrtc.TrackLocalStaticRTP
	audioTrack *webrtc.TrackLocalStaticRTP
}

// New 创建并返回一个新的 SFU 实例
func New() *SFU {
	return &SFU{
		subscribers: make(map[string]*webrtc.PeerConnection),
	}
}

// newWebRTCAPI 创建带 interceptor 的 WebRTC API 实例
func newWebRTCAPI() (*webrtc.API, error) {
	m := &webrtc.MediaEngine{}

	// 注册 H.264 编解码器（iOS 12 兼容）
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		},
		PayloadType: 102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}

	// 注册 Opus 音频编解码器（Chrome）
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}

	// 注册 PCMA（iOS 12 fallback）
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypePCMA,
			ClockRate: 8000,
			Channels:  1,
		},
		PayloadType: 8,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}

	// 注册 PCMU（iOS 12 fallback）
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypePCMU,
			ClockRate: 8000,
			Channels:  1,
		},
		PayloadType: 0,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}

	i := &interceptor.Registry{}

	// 注册 PLI 定期请求关键帧（publisher 用）
	intervalPLIFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		return nil, err
	}
	i.Add(intervalPLIFactory)

	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}

	return webrtc.NewAPI(
		webrtc.WithMediaEngine(m),
		webrtc.WithInterceptorRegistry(i),
	), nil
}

// peerConnectionConfig 返回 WebRTC 连接配置（使用 Google STUN）
func peerConnectionConfig() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
}

// HandlePublish 处理 OBS 的 WHIP 推流请求
// 创建 publisher PeerConnection，OnTrack 时将 RTP 转发给所有 subscriber
func (s *SFU) HandlePublish(offerSDP string) (string, error) {
	api, err := newWebRTCAPI()
	if err != nil {
		return "", err
	}

	pc, err := api.NewPeerConnection(peerConnectionConfig())
	if err != nil {
		return "", err
	}

	// 接收 video 和 audio
	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		return "", err
	}
	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		return "", err
	}

	// OnTrack：收到 publisher 的媒体轨道，转发给所有 subscriber
	pc.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("收到 publisher 轨道: %s", remoteTrack.Kind())

		var localTrack *webrtc.TrackLocalStaticRTP
		var err error

		switch remoteTrack.Kind() {
		case webrtc.RTPCodecTypeVideo:
			localTrack, err = webrtc.NewTrackLocalStaticRTP(
				remoteTrack.Codec().RTPCodecCapability, "video", streamID,
			)
			if err != nil {
				log.Printf("创建本地 video track 失败: %v", err)
				return
			}
			s.mu.Lock()
			s.videoTrack = localTrack
			s.mu.Unlock()

		case webrtc.RTPCodecTypeAudio:
			localTrack, err = webrtc.NewTrackLocalStaticRTP(
				remoteTrack.Codec().RTPCodecCapability, "audio", streamID,
			)
			if err != nil {
				log.Printf("创建本地 audio track 失败: %v", err)
				return
			}
			s.mu.Lock()
			s.audioTrack = localTrack
			s.mu.Unlock()

		default:
			return
		}

		// 将 RTP 包写入本地 track，所有 subscriber 都会收到
		go func() {
			buf := make([]byte, 1500)
			for {
				n, _, readErr := remoteTrack.Read(buf)
				if readErr != nil {
					if readErr != io.EOF {
						log.Printf("读取 RTP 包错误: %v", readErr)
					}
					return
				}
				if _, writeErr := localTrack.Write(buf[:n]); writeErr != nil {
					if writeErr != io.EOF {
						log.Printf("写入本地 track 错误: %v", writeErr)
					}
					return
				}
			}
		}()
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("Publisher ICE 状态: %s", state)
		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateClosed {
			s.mu.Lock()
			s.publisher = nil
			s.mu.Unlock()
		}
	})

	// 设置 offer
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offerSDP}
	if err = pc.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	// 等待 ICE 收集完成（Trickle ICE 简化为 ICE complete）
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(answer); err != nil {
		return "", err
	}
	<-gatherComplete

	s.mu.Lock()
	s.publisher = pc
	s.mu.Unlock()

	return pc.LocalDescription().SDP, nil
}

// HandleSubscribe 处理浏览器的 WHEP 拉流请求
// 创建 subscriber PeerConnection，添加 video + audio track
func (s *SFU) HandleSubscribe(offerSDP string) (answerSDP string, subID string, err error) {
	api, err := newWebRTCAPI()
	if err != nil {
		return "", "", err
	}

	pc, err := api.NewPeerConnection(peerConnectionConfig())
	if err != nil {
		return "", "", err
	}

	subID = uuid.New().String()

	// 添加 video track（如果 publisher 已经有了）
	s.mu.RLock()
	videoTrack := s.videoTrack
	audioTrack := s.audioTrack
	s.mu.RUnlock()

	if videoTrack != nil {
		if _, err = pc.AddTrack(videoTrack); err != nil {
			return "", "", err
		}
	} else {
		// 占位 transceiver，等 publisher 推流后再绑定
		if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendonly,
		}); err != nil {
			return "", "", err
		}
	}

	if audioTrack != nil {
		if _, err = pc.AddTrack(audioTrack); err != nil {
			return "", "", err
		}
	} else {
		if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendonly,
		}); err != nil {
			return "", "", err
		}
	}

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("Subscriber[%s] ICE 状态: %s", subID, state)
		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateClosed {
			s.RemoveSubscriber(subID)
		}
	})

	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offerSDP}
	if err = pc.SetRemoteDescription(offer); err != nil {
		return "", "", err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return "", "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(answer); err != nil {
		return "", "", err
	}
	<-gatherComplete

	s.mu.Lock()
	s.subscribers[subID] = pc
	count := len(s.subscribers)
	s.mu.Unlock()

	log.Printf("新 subscriber 加入: %s，当前总数: %d", subID, count)
	return pc.LocalDescription().SDP, subID, nil
}

// RemoveSubscriber 移除指定 subscriber 并关闭其连接
func (s *SFU) RemoveSubscriber(subID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pc, ok := s.subscribers[subID]; ok {
		pc.Close()
		delete(s.subscribers, subID)
		log.Printf("Subscriber[%s] 已移除，当前总数: %d", subID, len(s.subscribers))
	}
}

// Reset 清空 publisher 和所有 subscriber（OBS 断开推流时调用）
func (s *SFU) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.publisher != nil {
		s.publisher.Close()
		s.publisher = nil
	}
	for id, pc := range s.subscribers {
		pc.Close()
		delete(s.subscribers, id)
	}
	s.videoTrack = nil
	s.audioTrack = nil
	log.Println("SFU 已重置")
}

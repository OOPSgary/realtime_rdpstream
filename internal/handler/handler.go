// Package handler 提供 HTTP 处理器，实现 WHIP 和 WHEP 协议端点
package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"realtime_rdpstream/internal/sfu"
)

// Handler 封装 SFU 引用，提供 HTTP 处理方法
type Handler struct {
	sfu *sfu.SFU
}

// New 创建新的 Handler
func New(s *sfu.SFU) *Handler {
	return &Handler{sfu: s}
}

// HandleWHIP 处理 OBS 的 WHIP 推流请求
// POST /whip - body 为 offer SDP，返回 answer SDP
func (h *Handler) HandleWHIP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	offerSDP := strings.TrimSpace(string(body))
	if offerSDP == "" {
		http.Error(w, "offer SDP 不能为空", http.StatusBadRequest)
		return
	}

	log.Printf("收到 WHIP 推流请求，SDP 长度: %d", len(offerSDP))

	answerSDP, err := h.sfu.HandlePublish(offerSDP)
	if err != nil {
		log.Printf("HandlePublish 错误: %v", err)
		http.Error(w, fmt.Sprintf("建立推流连接失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Location", "/whip")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, answerSDP)
}

// HandleWHIPDelete 处理 OBS 停止推流请求
// DELETE /whip - 重置 SFU
func (h *Handler) HandleWHIPDelete(w http.ResponseWriter, r *http.Request) {
	log.Println("收到 WHIP DELETE 请求，重置 SFU")
	h.sfu.Reset()
	w.WriteHeader(http.StatusOK)
}

// HandleWHEP 处理浏览器的 WHEP 拉流请求
// POST /whep - body 为 offer SDP，返回 answer SDP，Header 含 Location
func (h *Handler) HandleWHEP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	offerSDP := strings.TrimSpace(string(body))
	if offerSDP == "" {
		http.Error(w, "offer SDP 不能为空", http.StatusBadRequest)
		return
	}

	log.Printf("收到 WHEP 拉流请求，SDP 长度: %d", len(offerSDP))

	answerSDP, subID, err := h.sfu.HandleSubscribe(offerSDP)
	if err != nil {
		log.Printf("HandleSubscribe 错误: %v", err)
		http.Error(w, fmt.Sprintf("建立拉流连接失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Location", "/whep/"+subID)
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, answerSDP)
}

package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"realtime_rdpstream/internal/handler"
	"realtime_rdpstream/internal/sfu"
)

//go:embed frontend/dist
var frontendDist embed.FS

func main() {
	// 初始化 SFU
	s := sfu.New()
	h := handler.New(s)

	mux := http.NewServeMux()

	// WHIP：OBS 推流端点
	mux.HandleFunc("POST /whip", h.HandleWHIP)
	mux.HandleFunc("DELETE /whip", h.HandleWHIPDelete)

	// WHEP：浏览器拉流端点
	mux.HandleFunc("POST /whep", h.HandleWHEP)

	// 静态文件服务（生产模式）
	distFS, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		log.Fatalf("无法加载前端静态文件: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(distFS)))

	log.Println("服务启动，监听 :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("服务器错误: %v", err)
	}
}

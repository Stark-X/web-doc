package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/xiaofengguo/web-doc/api/internal/model"
	"github.com/xiaofengguo/web-doc/api/internal/storage"
	"github.com/xiaofengguo/web-doc/api/internal/watcher"
	"gorm.io/gorm"
)

type Handler struct {
	DB              *gorm.DB
	Storage         *storage.Storage
	Hub             *watcher.Hub
	JWTSecret       string
	DisableRegister bool
	ShareBaseURL    string

	wsUpgrader websocket.Upgrader
}

func New(db *gorm.DB, st *storage.Storage, hub *watcher.Hub) *Handler {
	return &Handler{
		DB: db, Storage: st, Hub: hub,
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// 跨域 WS 由前置网关/CORS 控制；这里允许任意来源升级。
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ---------- 节点（文件夹/文档）管理 ----------

type createNodeReq struct {
	ParentID *string `json:"parentId"`
	Type     string  `json:"type"` // folder | doc
	Title    string  `json:"title"`
	HTML     string  `json:"html"` // 仅 type=doc 时使用，纯 HTML 源码
}

func (h *Handler) CreateNode(c *gin.Context) {
	var req createNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if req.Type != "folder" && req.Type != "doc" {
		badRequest(c, "type must be folder or doc")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = "未命名"
	}
	n := model.Node{
		ID:         uuid.NewString(),
		ParentID:   req.ParentID,
		Type:       req.Type,
		Title:      req.Title,
		EntryFile:  "index.html",
		Visibility: "private",
	}
	if req.Type == "doc" {
		if err := h.Storage.CreateDoc(n.ID, req.HTML); err != nil {
			serverError(c, err)
			return
		}
		h.Hub.AddDocWatch(h.Storage.DocPath(n.ID))
		n.SizeBytes = h.Storage.DocSize(n.ID)
	}
	if err := h.DB.Create(&n).Error; err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *Handler) ListNodes(c *gin.Context) {
	var nodes []model.Node
	if err := h.DB.Order("type desc, sort_order asc, created_at asc").Find(&nodes).Error; err != nil {
		log.Printf("[web-doc api] ListNodes failed path=%s userID=%q username=%q error=%v", c.Request.URL.RequestURI(), getLocal(c, "userID"), getLocal(c, "username"), err)
		serverError(c, err)
		return
	}
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Type == "doc" {
			ids = append(ids, n.ID)
		}
	}
	log.Printf("[web-doc api] ListNodes ok path=%s userID=%q username=%q total=%d docIDs=%v", c.Request.URL.RequestURI(), getLocal(c, "userID"), getLocal(c, "username"), len(nodes), ids)
	c.JSON(http.StatusOK, gin.H{"items": nodes})
}

func (h *Handler) GetNode(c *gin.Context) {
	id := c.Param("id")
	var n model.Node
	if err := h.DB.First(&n, "id = ?", id).Error; err != nil {
		log.Printf("[web-doc api] GetNode not found path=%s id=%q userID=%q username=%q error=%v", c.Request.URL.RequestURI(), id, getLocal(c, "userID"), getLocal(c, "username"), err)
		notFound(c)
		return
	}
	if n.Type == "doc" {
		files, _ := h.Storage.ListFiles(n.ID)
		log.Printf("[web-doc api] GetNode ok path=%s id=%q title=%q type=%q visibility=%q userID=%q username=%q files=%v", c.Request.URL.RequestURI(), n.ID, n.Title, n.Type, n.Visibility, getLocal(c, "userID"), getLocal(c, "username"), files)
		c.JSON(http.StatusOK, gin.H{"node": n, "files": files})
		return
	}
	log.Printf("[web-doc api] GetNode ok path=%s id=%q title=%q type=%q visibility=%q userID=%q username=%q", c.Request.URL.RequestURI(), n.ID, n.Title, n.Type, n.Visibility, getLocal(c, "userID"), getLocal(c, "username"))
	c.JSON(http.StatusOK, gin.H{"node": n})
}

type updateNodeReq struct {
	Title      *string `json:"title"`
	ParentID   *string `json:"parentId"`
	Visibility *string `json:"visibility"`
	EntryFile  *string `json:"entryFile"`
}

func (h *Handler) UpdateNode(c *gin.Context) {
	id := c.Param("id")
	var n model.Node
	if err := h.DB.First(&n, "id = ?", id).Error; err != nil {
		notFound(c)
		return
	}
	var req updateNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if req.Title != nil {
		n.Title = *req.Title
	}
	if req.ParentID != nil {
		// 允许 ParentID 为 "" 表示移到根目录
		if *req.ParentID == "" {
			n.ParentID = nil
		} else {
			pid := *req.ParentID
			n.ParentID = &pid
		}
	}
	if req.Visibility != nil {
		n.Visibility = *req.Visibility
	}
	if req.EntryFile != nil {
		n.EntryFile = *req.EntryFile
	}
	if err := h.DB.Save(&n).Error; err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *Handler) DeleteNode(c *gin.Context) {
	id := c.Param("id")
	var n model.Node
	if err := h.DB.First(&n, "id = ?", id).Error; err != nil {
		notFound(c)
		return
	}
	// 文件夹：递归删除子节点
	if n.Type == "folder" {
		if err := h.deleteRecursive(id); err != nil {
			serverError(c, err)
			return
		}
	} else {
		_ = h.Storage.RemoveDoc(id)
		h.DB.Where("doc_id = ?", id).Delete(&model.Share{})
	}
	if err := h.DB.Delete(&n).Error; err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) deleteRecursive(parentID string) error {
	var children []model.Node
	if err := h.DB.Where("parent_id = ?", parentID).Find(&children).Error; err != nil {
		return err
	}
	for _, ch := range children {
		if ch.Type == "folder" {
			if err := h.deleteRecursive(ch.ID); err != nil {
				return err
			}
		} else {
			_ = h.Storage.RemoveDoc(ch.ID)
			h.DB.Where("doc_id = ?", ch.ID).Delete(&model.Share{})
		}
		if err := h.DB.Delete(&ch).Error; err != nil {
			return err
		}
	}
	return nil
}

// ---------- 文档内容上传 / 单文件读写 ----------

// UploadHTML 直接保存 HTML 源码到 index.html
type uploadHTMLReq struct {
	HTML string `json:"html"`
	File string `json:"file"` // 默认 index.html
}

func (h *Handler) UploadHTML(c *gin.Context) {
	id := c.Param("id")
	var n model.Node
	if err := h.DB.First(&n, "id = ? AND type = 'doc'", id).Error; err != nil {
		notFound(c)
		return
	}
	var req uploadHTMLReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	file := req.File
	if file == "" {
		file = "index.html"
	}
	if err := h.Storage.WriteFile(id, file, []byte(req.HTML)); err != nil {
		serverError(c, err)
		return
	}
	n.SizeBytes = h.Storage.DocSize(id)
	h.DB.Save(&n)
	c.JSON(http.StatusOK, gin.H{"ok": true, "size": n.SizeBytes})
}

// UploadZip 上传 zip 解压到文档目录
func (h *Handler) UploadZip(c *gin.Context) {
	id := c.Param("id")
	var n model.Node
	if err := h.DB.First(&n, "id = ? AND type = 'doc'", id).Error; err != nil {
		notFound(c)
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "missing file")
		return
	}
	f, err := fh.Open()
	if err != nil {
		serverError(c, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		serverError(c, err)
		return
	}
	hasIndex, err := h.Storage.ExtractZip(id, bytesReaderAt(data), int64(len(data)))
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	n.SizeBytes = h.Storage.DocSize(id)
	// 如果用户已经设置过 entryFile 且文件存在，则直接沿用；否则若顶层无 index.html，告诉前端需要选择入口
	files, _ := h.Storage.ListFiles(id)
	needsEntry := false
	if !hasIndex {
		// 当前 entryFile 是否仍然有效
		if !h.Storage.FileExists(id, n.EntryFile) {
			needsEntry = true
		}
	} else {
		// 顶层有 index.html：自动设为 entry
		n.EntryFile = "index.html"
	}
	h.DB.Save(&n)
	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"size":       n.SizeBytes,
		"hasIndex":   hasIndex,
		"needsEntry": needsEntry,
		"files":      files,
	})
}

// GetFileContent 读取文档下指定文件文本内容（用于编辑器）
func (h *Handler) GetFileContent(c *gin.Context) {
	id := c.Param("id")
	sub := c.DefaultQuery("path", "index.html")
	full, err := h.Storage.ResolveSafe(id, sub)
	if err != nil {
		badRequest(c, "invalid path")
		return
	}
	data, err := readFileSafe(full)
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": sub, "content": string(data)})
}

// ---------- 静态资源服务 (/d/:id/*path, /p/:token/*path) ----------

// docSandboxCSP 与前端 iframe 的 sandbox 属性对齐，但不含 allow-same-origin：
// 文档以 opaque origin 运行，访问 localStorage / cookie 会抛 SecurityError，
// 上传文档中的恶意 JS 无法窃取已登录访问者的 JWT。
const docSandboxCSP = "sandbox allow-scripts allow-forms allow-popups allow-modals allow-downloads"

func (h *Handler) ServeDocAsset(c *gin.Context) {
	id := c.Param("id")
	sub := strings.TrimPrefix(c.Param("path"), "/")
	// 兼容历史行为：访问 /d/:id 或 /d/:id/ 时重定向到 index.html
	if sub == "" {
		c.Redirect(http.StatusFound, "/d/"+id+"/index.html")
		return
	}
	var n model.Node
	if err := h.DB.First(&n, "id = ? AND type = 'doc'", id).Error; err != nil {
		c.String(http.StatusNotFound, "Not Found")
		return
	}
	// 仅主站自己的编辑器 iframe（同源加载）豁免 sandbox，保住 ?p= 深链同步；
	// 其余场景（顶层直开、外站嵌入、不发 Sec-Fetch 头的旧浏览器）一律 sandbox，fail-closed。
	exempt := c.GetHeader("Sec-Fetch-Dest") == "iframe" && c.GetHeader("Sec-Fetch-Site") == "same-origin"
	h.serveDocFile(c, id, sub, !exempt)
}

// serveDocFile 输出文档目录下的单个文件（/d/ 与 /p/ 共用）。
// sandbox=true 时附带 CSP sandbox 头，使文档运行在 opaque origin。
func (h *Handler) serveDocFile(c *gin.Context, id, sub string, sandbox bool) {
	full, err := h.Storage.ResolveSafe(id, sub)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid path")
		return
	}
	// 设置 MIME 类型
	ext := strings.ToLower(filepath.Ext(full))
	if ct := mime.TypeByExtension(ext); ct != "" {
		c.Header("Content-Type", ct)
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	if sandbox {
		// 不能叠加 frame-ancestors：sandbox 后文档内部自嵌的 iframe 祖先是 opaque origin，
		// 会被 'self' 误杀；且 opaque origin 本身已阻断对主站的一切访问。
		c.Header("Content-Security-Policy", docSandboxCSP)
	} else {
		// 未 sandbox 的响应（编辑器 iframe）只允许同源嵌入，防外站嵌套
		c.Header("Content-Security-Policy", "frame-ancestors 'self'")
	}
	// 禁用浏览器/中间层缓存：文档内容会被实时编辑，必须每次拿最新版本
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	// 直接读文件并写入响应，确保拿到最新内容（避免任何中间缓存层）
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			c.String(http.StatusNotFound, "Not Found")
			return
		}
		log.Printf("[serveDocFile] read file failed id=%s sub=%s err=%v", id, sub, err)
		c.String(http.StatusInternalServerError, "read failed")
		return
	}
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(data)
}

// ServeSharedDocAsset 纯静态分享（/p/:token/*path）：
// 通过 share token 直接输出文档文件，无 React 外壳、无 iframe，删除 Share 记录即可撤销。
// 始终携带 CSP sandbox，访客打开的文档拿不到主站登录态。
func (h *Handler) ServeSharedDocAsset(c *gin.Context) {
	token := c.Param("token")
	var s model.Share
	if err := h.DB.Where("token = ?", token).First(&s).Error; err != nil {
		c.String(http.StatusNotFound, "Not Found")
		return
	}
	if s.ExpiresAt != nil && time.Now().After(*s.ExpiresAt) {
		c.String(http.StatusNotFound, "Not Found")
		return
	}
	var n model.Node
	if err := h.DB.First(&n, "id = ? AND type = 'doc'", s.DocID).Error; err != nil {
		c.String(http.StatusNotFound, "Not Found")
		return
	}
	sub := strings.TrimPrefix(c.Param("path"), "/")
	// 访问 /p/:token 或 /p/:token/ 时重定向到文档入口文件
	if sub == "" {
		entry := n.EntryFile
		if entry == "" {
			entry = "index.html"
		}
		c.Redirect(http.StatusFound, (&url.URL{Path: "/p/" + token + "/" + entry}).RequestURI())
		return
	}
	h.serveDocFile(c, n.ID, sub, true)
}

// ---------- 分享 ----------

func (h *Handler) CreateShare(c *gin.Context) {
	id := c.Param("id")
	var n model.Node
	if err := h.DB.First(&n, "id = ? AND type = 'doc'", id).Error; err != nil {
		notFound(c)
		return
	}
	// 复用已有未过期分享
	var existing model.Share
	if err := h.DB.Where("doc_id = ?", id).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, h.shareResponse(existing))
		return
	}
	tk := randomToken(12)
	s := model.Share{
		ID:        uuid.NewString(),
		DocID:     id,
		Token:     tk,
		CreatedAt: time.Now(),
	}
	if err := h.DB.Create(&s).Error; err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.shareResponse(s))
}

type shareResponse struct {
	ID                 string     `json:"id"`
	DocID              string     `json:"docId"`
	Token              string     `json:"token"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	StaticShareBaseURL string     `json:"staticShareBaseUrl,omitempty"`
}

func (h *Handler) shareResponse(s model.Share) shareResponse {
	return shareResponse{
		ID:                 s.ID,
		DocID:              s.DocID,
		Token:              s.Token,
		ExpiresAt:          s.ExpiresAt,
		CreatedAt:          s.CreatedAt,
		StaticShareBaseURL: h.ShareBaseURL,
	}
}

// GetShareInfo 通过 token 取得文档信息（前端 /s/:token 用）
func (h *Handler) GetShareInfo(c *gin.Context) {
	token := c.Param("token")
	var s model.Share
	if err := h.DB.Where("token = ?", token).First(&s).Error; err != nil {
		log.Printf("[web-doc api] GetShareInfo share not found path=%s token=%q userID=%q username=%q error=%v", c.Request.URL.RequestURI(), token, getLocal(c, "userID"), getLocal(c, "username"), err)
		notFound(c)
		return
	}
	var n model.Node
	if err := h.DB.First(&n, "id = ?", s.DocID).Error; err != nil {
		log.Printf("[web-doc api] GetShareInfo doc not found path=%s token=%q docID=%q userID=%q username=%q error=%v", c.Request.URL.RequestURI(), token, s.DocID, getLocal(c, "userID"), getLocal(c, "username"), err)
		notFound(c)
		return
	}
	log.Printf("[web-doc api] GetShareInfo ok path=%s token=%q shareID=%q docID=%q title=%q visibility=%q userID=%q username=%q", c.Request.URL.RequestURI(), token, s.ID, n.ID, n.Title, n.Visibility, getLocal(c, "userID"), getLocal(c, "username"))
	c.JSON(http.StatusOK, gin.H{
		"share": h.shareResponse(s),
		"doc":   n,
	})
}

// ---------- WebSocket 监听文档变更 ----------

func (h *Handler) WSDocWatch(c *gin.Context) {
	docID := c.Param("id")
	conn, err := h.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 失败时它内部已写过响应；这里只记录日志
		log.Printf("[web-doc ws] upgrade failed docID=%s err=%v", docID, err)
		return
	}
	defer conn.Close()

	ch := h.Hub.Subscribe(docID)
	defer h.Hub.Unsubscribe(docID, ch)

	// 单独 goroutine 读，避免 Conn 阻塞被关闭
	done := make(chan struct{})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(done)
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case ev := <-ch:
			if ev == "" {
				return
			}
			if err := conn.WriteJSON(gin.H{"type": ev}); err != nil {
				return
			}
		}
	}
}

// ---------- 工具函数 ----------

func badRequest(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": msg})
}
func notFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
}
func serverError(c *gin.Context, err error) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

// getLocal 等价于 fiber 的 c.Locals("xxx")，统一返回 string（便于日志格式化）。
func getLocal(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// 简单的 ReaderAt 包装
type byteReaderAt struct{ b []byte }

func (r *byteReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.b)) {
		return 0, errors.New("EOF")
	}
	n := copy(p, r.b[off:])
	if n < len(p) {
		return n, errors.New("short read")
	}
	return n, nil
}

func bytesReaderAt(b []byte) *byteReaderAt { return &byteReaderAt{b: b} }

package controllers

import (
	"encoding/json"
	"encoding/base64"
	"net/http"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kliro/models"
	"kliro/utils"
)

var allowedBlogCategories = map[string]struct{}{
	"bank":      {},
	"insurance": {},
	"avia":      {},
	"hotel":     {},
}

type BlogPostRequest struct {
	Category    string   `json:"category" binding:"required"`
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description" binding:"required"`
	Photos      []string `json:"photos"` // max 10
	PhotosB64   []string `json:"photos_base64"` // optional, max 10, base64 data URLs or raw base64
	Links       []string `json:"links"`  // max 10
}

type BlogController struct{}

func NewBlogController() *BlogController {
	return &BlogController{}
}

const maxUploadSizeBytes = 10 * 1024 * 1024 // 10 MB

func normalizeCategory(c string) string {
	return strings.ToLower(strings.TrimSpace(c))
}

func validateBlogRequest(req *BlogPostRequest) (bool, string) {
	req.Category = normalizeCategory(req.Category)
	if _, ok := allowedBlogCategories[req.Category]; !ok {
		return false, "category must be one of: bank, insurance, avia, hotel"
	}
	if len(req.Photos) > 10 {
		return false, "photos length must be <= 10"
	}
	if len(req.Links) > 10 {
		return false, "links length must be <= 10"
	}
	return true, ""
}

func encodeStringSlice(v []string) (string, error) {
	if v == nil {
		return "[]", nil
	}
	b, err := json.Marshal(v)
	return string(b), err
}

func decodeStringSlice(j string) []string {
	if len(j) == 0 {
		return []string{}
	}
	var out []string
	_ = json.Unmarshal([]byte(j), &out)
	return out
}

func isMultipart(c *gin.Context) bool {
	ct := c.ContentType()
	return strings.HasPrefix(ct, "multipart/form-data")
}

func detectExtFromBytes(b []byte) (string, string) {
	ct := http.DetectContentType(b)
	switch ct {
	case "image/jpeg":
		return ".jpg", ct
	case "image/png":
		return ".png", ct
	default:
		return "", ct
	}
}

func saveImagePreserveQuality(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Ограничение по размеру
	if file.Size > maxUploadSizeBytes {
		return "", gin.Error{Err: err, Type: gin.ErrorTypeBind}
	}

	// Создаём директорию кэша
	cacheDir := filepath.Join("uploads", "blog", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	// Имя файла
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	filename := strconv.FormatInt(time.Now().UnixNano(), 10) + ext
	fullPath := filepath.Join(cacheDir, filename)

	// Сохранение без перекодирования (без потери качества)
	out, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return "", err
	}

	// Публичный URL
	publicURL := "/uploads/blog/cache/" + filename
	return publicURL, nil
}

func saveBase64ImagePreserveQuality(b64 string) (string, error) {
	// Поддержка data URL: data:image/jpeg;base64,XXXXX
	if idx := strings.Index(b64, ","); idx != -1 && strings.HasPrefix(strings.ToLower(b64[:idx]), "data:") {
		b64 = b64[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		// некоторые клиенты используют StdEncoding без padding — пробуем RawStdEncoding
		data, err = base64.RawStdEncoding.DecodeString(b64)
		if err != nil {
			return "", err
		}
	}
	if len(data) > maxUploadSizeBytes {
		return "", gin.Error{Err: err, Type: gin.ErrorTypeBind}
	}
	ext, _ := detectExtFromBytes(data)
	if ext == "" {
		return "", gin.Error{Err: err, Type: gin.ErrorTypeBind}
	}
	cacheDir := filepath.Join("uploads", "blog", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	filename := strconv.FormatInt(time.Now().UnixNano(), 10) + ext
	fullPath := filepath.Join(cacheDir, filename)
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", err
	}
	return "/uploads/blog/cache/" + filename, nil
}

func collectLinksFromMultipart(c *gin.Context) []string {
	links := c.PostFormArray("links")
	if len(links) == 0 {
		if raw := c.PostForm("links_json"); raw != "" {
			var arr []string
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				return arr
			}
		}
	}
	return links
}

// POST /blog
func (bc *BlogController) Create(c *gin.Context) {
	db := utils.GetDB()

	if isMultipart(c) {
		// Multipart режим: поля берём из form-data, фото загружаем из файлов
		req := BlogPostRequest{
			Category:    c.PostForm("category"),
			Title:       c.PostForm("title"),
			Description: c.PostForm("description"),
			Links:       collectLinksFromMultipart(c),
		}

		// Валидация категории, title/description, лимитов
		if strings.TrimSpace(req.Category) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Description) == "" {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "category, title, description are required"})
			return
		}
		if ok, msg := validateBlogRequest(&req); !ok {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": msg})
			return
		}

		// Файлы: поддерживаем ключи files и/или photos
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid multipart form"})
			return
		}
		fileHeaders := append(form.File["files"], form.File["photos"]...)
		if len(fileHeaders) > 10 {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "up to 10 images allowed"})
			return
		}
		uploadedURLs := make([]string, 0, len(fileHeaders))
		for _, fh := range fileHeaders {
			// Проверка расширения
			ext := strings.ToLower(filepath.Ext(fh.Filename))
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
				c.JSON(400, gin.H{"result": nil, "success": false, "error": "only jpg and png are allowed"})
				return
			}
			if fh.Size > maxUploadSizeBytes {
				c.JSON(400, gin.H{"result": nil, "success": false, "error": "file too large (max 10MB)"})
				return
			}
			url, err := saveImagePreserveQuality(fh)
			if err != nil {
				c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to save image"})
				return
			}
			uploadedURLs = append(uploadedURLs, url)
		}

		photosJSON, _ := encodeStringSlice(uploadedURLs)
		linksJSON, _ := encodeStringSlice(req.Links)
		post := models.BlogPost{
			Category:    req.Category,
			Title:       req.Title,
			Description: req.Description,
			Photos:      photosJSON,
			Links:       linksJSON,
		}
		if err := db.Create(&post).Error; err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create blog post"})
			return
		}
		c.JSON(201, gin.H{"result": bc.toResponse(post), "success": true})
		return
	}

	// JSON режим (старый)
	var req BlogPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if ok, msg := validateBlogRequest(&req); !ok {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": msg})
		return
	}
	if len(req.Photos) > 10 || len(req.PhotosB64) > 10 {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "photos/photos_base64 length must be <= 10"})
		return
	}

	// Если прислан base64, сохраняем и подменяем photos на URL
	uploadedURLs := make([]string, 0, len(req.PhotosB64)+len(req.Photos))
	if len(req.PhotosB64) > 0 {
		if len(req.PhotosB64) > 10 {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "up to 10 images allowed"})
			return
		}
		for _, b64 := range req.PhotosB64 {
			url, err := saveBase64ImagePreserveQuality(b64)
			if err != nil {
				c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid base64 image"})
				return
			}
			uploadedURLs = append(uploadedURLs, url)
		}
	}
	// Добавляем уже готовые URL, если есть
	if len(req.Photos) > 0 {
		uploadedURLs = append(uploadedURLs, req.Photos...)
	}

	photosJSON, err := encodeStringSlice(req.Photos)
	if err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid photos"})
		return
	}
	// Если были base64, заменяем photosJSON на загруженные URL
	if len(uploadedURLs) > 0 {
		if len(uploadedURLs) > 10 {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "up to 10 images allowed"})
			return
		}
		if photosJSON, err = encodeStringSlice(uploadedURLs); err != nil {
			c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid photos"})
			return
		}
	}
	linksJSON, err := encodeStringSlice(req.Links)
	if err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid links"})
		return
	}
	post := models.BlogPost{
		Category:    req.Category,
		Title:       req.Title,
		Description: req.Description,
		Photos:      photosJSON,
		Links:       linksJSON,
	}
	if err := db.Create(&post).Error; err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to create blog post"})
		return
	}
	c.JSON(201, gin.H{"result": bc.toResponse(post), "success": true})
}

// PUT /blog/:id
func (bc *BlogController) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid id"})
		return
	}
	var req BlogPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if ok, msg := validateBlogRequest(&req); !ok {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": msg})
		return
	}
	db := utils.GetDB()
	var post models.BlogPost
	if err := db.First(&post, id).Error; err != nil {
		c.JSON(404, gin.H{"result": nil, "success": false, "error": "blog post not found"})
		return
	}
	photosJSON, err := encodeStringSlice(req.Photos)
	if err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid photos"})
		return
	}
	linksJSON, err := encodeStringSlice(req.Links)
	if err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid links"})
		return
	}
	post.Category = req.Category
	post.Title = req.Title
	post.Description = req.Description
	post.Photos = photosJSON
	post.Links = linksJSON
	if err := db.Save(&post).Error; err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to update blog post"})
		return
	}
	c.JSON(200, gin.H{"result": bc.toResponse(post), "success": true})
}

// DELETE /blog/:id
func (bc *BlogController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid id"})
		return
	}
	db := utils.GetDB()
	if err := db.Delete(&models.BlogPost{}, id).Error; err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to delete blog post"})
		return
	}
	c.JSON(200, gin.H{"result": gin.H{"id": id}, "success": true})
}

// GET /blog
func (bc *BlogController) List(c *gin.Context) {
	db := utils.GetDB()
	var posts []models.BlogPost
	if err := db.Order("created_at desc").Find(&posts).Error; err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to fetch blog posts"})
		return
	}
	resp := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		resp = append(resp, bc.toResponse(p))
	}
	c.JSON(200, gin.H{"result": resp, "success": true})
}

// GET /blog/category/:category
func (bc *BlogController) ListByCategory(c *gin.Context) {
	category := normalizeCategory(c.Param("category"))
	if _, ok := allowedBlogCategories[category]; !ok {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid category"})
		return
	}
	db := utils.GetDB()
	var posts []models.BlogPost
	if err := db.Where("category = ?", category).Order("created_at desc").Find(&posts).Error; err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to fetch blog posts"})
		return
	}
	resp := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		resp = append(resp, bc.toResponse(p))
	}
	c.JSON(200, gin.H{"result": resp, "success": true})
}

func (bc *BlogController) toResponse(p models.BlogPost) gin.H {
	return gin.H{
		"id":          p.ID,
		"category":    p.Category,
		"title":       p.Title,
		"description": p.Description,
		"photos":      decodeStringSlice(p.Photos),
		"links":       decodeStringSlice(p.Links),
		"createdAt":   p.CreatedAt,
		"updatedAt":   p.UpdatedAt,
	}
}



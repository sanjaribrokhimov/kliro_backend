package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"mime/multipart"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"kliro/models"
	"kliro/utils"
)

type localizedPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

type blogPostPayload struct {
	Category     string           `json:"category" binding:"required"`
	Tags         []string         `json:"tags"`
	PhotosBase64 []string         `json:"photos_base64"`
	Uz           localizedPayload `json:"uz"`
	Oz           localizedPayload `json:"oz"`
	Ru           localizedPayload `json:"ru"`
	En           localizedPayload `json:"en"`
}

type BlogController struct{}

func NewBlogController() *BlogController {
	return &BlogController{}
}

func normalizeCategory(c string) string {
	return strings.ToLower(strings.TrimSpace(c))
}

func getLang(c *gin.Context) string {
	lang := strings.ToLower(strings.TrimSpace(c.GetHeader("Lang")))
	switch lang {
	case "uz", "oz", "ru", "en":
		return lang
	default:
		return "ru"
	}
}

func jsonFrom(v any) datatypes.JSON {
	b, _ := json.Marshal(v)
	return datatypes.JSON(b)
}

func parseJSONStrings(j datatypes.JSON) []string {
	if len(j) == 0 {
		return []string{}
	}
	var out []string
	_ = json.Unmarshal(j, &out)
	return out
}

func parseJSONLocalized(j datatypes.JSON) localizedPayload {
	if len(j) == 0 {
		return localizedPayload{}
	}
	var out localizedPayload
	_ = json.Unmarshal(j, &out)
	return out
}

func pickLocalized(p models.BlogPost, lang string) localizedPayload {
	switch strings.ToLower(lang) {
	case "uz":
		lp := parseJSONLocalized(p.Uz)
		if lp.Title != "" || lp.Description != "" || lp.Content != "" {
			return lp
		}
	case "oz":
		lp := parseJSONLocalized(p.Oz)
		if lp.Title != "" || lp.Description != "" || lp.Content != "" {
			return lp
		}
	case "en":
		lp := parseJSONLocalized(p.En)
		if lp.Title != "" || lp.Description != "" || lp.Content != "" {
			return lp
		}
	}
	if lp := parseJSONLocalized(p.Ru); lp.Title != "" || lp.Description != "" || lp.Content != "" {
		return lp
	}
	if lp := parseJSONLocalized(p.En); lp.Title != "" || lp.Description != "" || lp.Content != "" {
		return lp
	}
	if lp := parseJSONLocalized(p.Uz); lp.Title != "" || lp.Description != "" || lp.Content != "" {
		return lp
	}
	return parseJSONLocalized(p.Oz)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func transliterate(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	repl := map[rune]string{
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d",
		'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i",
		'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
		'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t",
		'у': "u", 'ф': "f", 'х': "h", 'ц': "c", 'ч': "ch",
		'ш': "sh", 'щ': "sh", 'ъ': "", 'ы': "y", 'ь': "",
		'э': "e", 'ю': "yu", 'я': "ya",
		'ў': "u", 'қ': "q", 'ғ': "g", 'ҳ': "h", 'ґ': "g",
		'ä': "a", 'ö': "o", 'ü': "u",
	}
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == ' ' || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		if m, ok := repl[r]; ok {
			b.WriteString(m)
			continue
		}
		if unicode.IsSpace(r) {
			b.WriteRune(' ')
			continue
		}
	}
	return b.String()
}

func slugify(title string) string {
	base := transliterate(title)
	base = nonAlnum.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return "post"
	}
	return base
}

func generateUniqueAlias(db *gorm.DB, base string, excludeID uint) (string, error) {
	alias := base
	i := 1
	for {
		var count int64
		q := db.Model(&models.BlogPost{}).Where("alias = ?", alias)
		if excludeID > 0 {
			q = q.Where("id <> ?", excludeID)
		}
		if err := q.Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return alias, nil
		}
		i++
		alias = fmt.Sprintf("%s-%d", base, i)
	}
}

// сохраняет файл блога в ./uploads/blog и возвращает URL вида /uploads/blog/<filename>
func saveBlogUploadedFile(c *gin.Context, file *multipart.FileHeader) (string, error) {
	dstDir := "./uploads/blog"
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
	dstPath := filepath.Join(dstDir, filename)

	if err := c.SaveUploadedFile(file, dstPath); err != nil {
		return "", err
	}

	return "/uploads/blog/" + filename, nil
}

// POST /blog/upload-photo
// multipart/form-data, поле "file"
// Возвращает URL вида /uploads/blog/<filename>
func (bc *BlogController) UploadPhoto(c *gin.Context) {
	// Ограничение по размеру файла, например 10 МБ
	const maxUploadSize = 10 << 20 // 10 MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "result": nil, "error": "file is required"})
		return
	}

	url, err := saveBlogUploadedFile(c, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "result": nil, "error": "failed to save file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result": gin.H{
			"url": url,
		},
	})
}

// POST /blog/create
func (bc *BlogController) Create(c *gin.Context) {
	var req blogPostPayload

	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// multipart-вариант: поля приходят как form-data, картинка(и) как file/files
		req.Category = c.PostForm("category")

		// tags как строка "tag1,tag2"
		tagsStr := strings.TrimSpace(c.PostForm("tags"))
		if tagsStr != "" {
			parts := strings.Split(tagsStr, ",")
			for _, t := range parts {
				t = strings.TrimSpace(t)
				if t != "" {
					req.Tags = append(req.Tags, t)
				}
			}
		}

		// локализованные поля
		req.Uz = localizedPayload{
			Title:       c.PostForm("uz_title"),
			Description: c.PostForm("uz_description"),
			Content:     c.PostForm("uz_content"),
		}
		req.Oz = localizedPayload{
			Title:       c.PostForm("oz_title"),
			Description: c.PostForm("oz_description"),
			Content:     c.PostForm("oz_content"),
		}
		req.Ru = localizedPayload{
			Title:       c.PostForm("ru_title"),
			Description: c.PostForm("ru_description"),
			Content:     c.PostForm("ru_content"),
		}
		req.En = localizedPayload{
			Title:       c.PostForm("en_title"),
			Description: c.PostForm("en_description"),
			Content:     c.PostForm("en_content"),
		}

		// файлы: поддерживаем files[] и один file
		var photos []string
		if form, err := c.MultipartForm(); err == nil && form.File != nil {
			if files, ok := form.File["files"]; ok {
				for _, fh := range files {
					url, err := saveBlogUploadedFile(c, fh)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "result": nil, "error": "failed to save file"})
						return
					}
					photos = append(photos, url)
				}
			}
		}
		if len(photos) == 0 {
			if fh, err := c.FormFile("file"); err == nil {
				url, err := saveBlogUploadedFile(c, fh)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"success": false, "result": nil, "error": "failed to save file"})
					return
				}
				photos = append(photos, url)
			}
		}
		req.PhotosBase64 = photos
	} else {
		// старый JSON-вариант остаётся рабочим
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"success": false, "result": nil, "error": "invalid request"})
			return
		}
	}

	req.Category = normalizeCategory(req.Category)
	if len(req.PhotosBase64) > 50 {
		c.JSON(400, gin.H{"success": false, "result": nil, "error": "photos_base64 must be <= 50"})
		return
	}
	db := utils.GetDB()
	baseTitle := req.Ru.Title
	if baseTitle == "" {
		if req.En.Title != "" {
			baseTitle = req.En.Title
		} else if req.Uz.Title != "" {
			baseTitle = req.Uz.Title
		} else {
			baseTitle = req.Oz.Title
		}
	}
	baseDescription := req.Ru.Description
	if baseDescription == "" {
		if req.En.Description != "" {
			baseDescription = req.En.Description
		} else if req.Uz.Description != "" {
			baseDescription = req.Uz.Description
		} else {
			baseDescription = req.Oz.Description
		}
	}
	if baseDescription == "" {
		baseDescription = baseTitle
	}

	base := slugify(baseTitle)
	alias, err := generateUniqueAlias(db, base, 0)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "result": nil, "error": "failed to generate alias"})
		return
	}
	post := models.BlogPost{
		Category:    req.Category,
		Title:       baseTitle,
		Description: baseDescription,
		Tags:        jsonFrom(req.Tags),
		PhotosBase64: jsonFrom(req.PhotosBase64),
		Uz:          jsonFrom(req.Uz),
		Oz:          jsonFrom(req.Oz),
		Ru:          jsonFrom(req.Ru),
		En:          jsonFrom(req.En),
		Views:       0,
		Likes:       0,
		Alias:       alias,
	}
	if err := db.Create(&post).Error; err != nil {
		c.JSON(500, gin.H{"success": false, "result": nil, "error": "failed to create post"})
		return
	}
	c.JSON(201, gin.H{"success": true, "result": bc.toItem(c, post)})
}

// PUT /blog/update/:id
func (bc *BlogController) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"success": false, "result": nil, "error": "invalid id"})
		return
	}
	var req blogPostPayload

	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// multipart-вариант обновления
		req.Category = c.PostForm("category")

		tagsStr := strings.TrimSpace(c.PostForm("tags"))
		if tagsStr != "" {
			parts := strings.Split(tagsStr, ",")
			for _, t := range parts {
				t = strings.TrimSpace(t)
				if t != "" {
					req.Tags = append(req.Tags, t)
				}
			}
		}

		req.Uz = localizedPayload{
			Title:       c.PostForm("uz_title"),
			Description: c.PostForm("uz_description"),
			Content:     c.PostForm("uz_content"),
		}
		req.Oz = localizedPayload{
			Title:       c.PostForm("oz_title"),
			Description: c.PostForm("oz_description"),
			Content:     c.PostForm("oz_content"),
		}
		req.Ru = localizedPayload{
			Title:       c.PostForm("ru_title"),
			Description: c.PostForm("ru_description"),
			Content:     c.PostForm("ru_content"),
		}
		req.En = localizedPayload{
			Title:       c.PostForm("en_title"),
			Description: c.PostForm("en_description"),
			Content:     c.PostForm("en_content"),
		}

		var photos []string
		if form, err := c.MultipartForm(); err == nil && form.File != nil {
			if files, ok := form.File["files"]; ok {
				for _, fh := range files {
					url, err := saveBlogUploadedFile(c, fh)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "result": nil, "error": "failed to save file"})
						return
					}
					photos = append(photos, url)
				}
			}
		}
		if len(photos) == 0 {
			if fh, err := c.FormFile("file"); err == nil {
				url, err := saveBlogUploadedFile(c, fh)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"success": false, "result": nil, "error": "failed to save file"})
					return
				}
				photos = append(photos, url)
			}
		}
		req.PhotosBase64 = photos
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"success": false, "result": nil, "error": "invalid request"})
			return
		}
	}
	req.Category = normalizeCategory(req.Category)
	db := utils.GetDB()
	var post models.BlogPost
	if err := db.First(&post, id).Error; err != nil {
		c.JSON(404, gin.H{"success": false, "result": nil, "error": "post not found"})
		return
	}
	baseTitle := req.Ru.Title
	if baseTitle == "" {
		if req.En.Title != "" {
			baseTitle = req.En.Title
		} else if req.Uz.Title != "" {
			baseTitle = req.Uz.Title
		} else {
			baseTitle = req.Oz.Title
		}
	}
	baseDescription := req.Ru.Description
	if baseDescription == "" {
		if req.En.Description != "" {
			baseDescription = req.En.Description
		} else if req.Uz.Description != "" {
			baseDescription = req.Uz.Description
		} else {
			baseDescription = req.Oz.Description
		}
	}
	if baseDescription == "" {
		baseDescription = baseTitle
	}
	base := slugify(baseTitle)
	alias, err := generateUniqueAlias(db, base, post.ID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "result": nil, "error": "failed to generate alias"})
		return
	}
	post.Category = req.Category
	post.Title = baseTitle
	post.Description = baseDescription
	post.Tags = jsonFrom(req.Tags)
	post.PhotosBase64 = jsonFrom(req.PhotosBase64)
	post.Uz = jsonFrom(req.Uz)
	post.Oz = jsonFrom(req.Oz)
	post.Ru = jsonFrom(req.Ru)
	post.En = jsonFrom(req.En)
	post.Alias = alias
	if err := db.Save(&post).Error; err != nil {
		c.JSON(500, gin.H{"success": false, "result": nil, "error": "failed to update post"})
		return
	}
	c.JSON(200, gin.H{"success": true, "result": bc.toItem(c, post)})
}

// DELETE /blog/:id
func (bc *BlogController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"success": false, "result": nil, "error": "invalid id"})
		return
	}
	db := utils.GetDB()
	if err := db.Delete(&models.BlogPost{}, id).Error; err != nil {
		c.JSON(500, gin.H{"success": false, "result": nil, "error": "failed to delete post"})
		return
	}
	c.JSON(200, gin.H{"success": true, "result": gin.H{"id": id}})
}

// GET /blog
// Query: ?category=bank&page=1&page_size=20&search=a&lang=ru
func (bc *BlogController) List(c *gin.Context) {
	db := utils.GetDB()
	category := normalizeCategory(c.Query("category"))
	search := strings.TrimSpace(c.Query("search"))
	lang := getLang(c)
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := c.Query("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			pageSize = n
		}
	}
	offset := (page - 1) * pageSize
	q := db.Model(&models.BlogPost{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if search != "" {
		p := "%" + strings.ToLower(search) + "%"
		q = q.Where("(LOWER(alias) LIKE ? OR LOWER(ru::text) LIKE ? OR LOWER(en::text) LIKE ? OR LOWER(uz::text) LIKE ? OR LOWER(oz::text) LIKE ?)", p, p, p, p, p)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"success": false, "result": nil, "error": "failed to count posts"})
		return
	}
	var posts []models.BlogPost
	if err := q.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&posts).Error; err != nil {
		c.JSON(500, gin.H{"success": false, "result": nil, "error": "failed to fetch posts"})
		return
	}
	items := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		items = append(items, bc.toItemWithLang(p, lang))
	}
	c.JSON(200, gin.H{
		"success": true,
		"result": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total_count": total,
			"data":        items,
		},
	})
}

// GET /blog/:id
func (bc *BlogController) GetByID(c *gin.Context) {
	db := utils.GetDB()
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"success": false, "result": nil, "error": "invalid id"})
		return
	}
	lang := getLang(c)
	var post models.BlogPost
	if err := db.First(&post, id).Error; err != nil {
		c.JSON(404, gin.H{"success": false, "result": nil, "error": "post not found"})
		return
	}
	c.JSON(200, gin.H{"success": true, "result": bc.toItemWithLang(post, lang)})
	}

func (bc *BlogController) toItem(c *gin.Context, p models.BlogPost) gin.H {
	lang := getLang(c)
	return bc.toItemWithLang(p, lang)
}

func (bc *BlogController) toItemWithLang(p models.BlogPost, lang string) gin.H {
	lp := pickLocalized(p, lang)
	return gin.H{
		"id":            p.ID,
		"created_at":    p.CreatedAt.Format(time.RFC3339),
		"updated_at":    p.UpdatedAt.Format(time.RFC3339),
		"category":      p.Category,
		"title":         lp.Title,
		"tags":          parseJSONStrings(p.Tags),
		"description":   lp.Description,
		"content":       lp.Content,
		"photos_base64": parseJSONStrings(p.PhotosBase64),
		"views":         p.Views,
		"likes":         p.Likes,
		"alias":         p.Alias,
	}
}



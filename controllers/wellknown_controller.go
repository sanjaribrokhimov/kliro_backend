package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"kliro/config"
)

// WellKnownController отдаёт файлы для Android App Links и iOS Universal Links
type WellKnownController struct {
	cfg *config.Config
}

// NewWellKnownController создаёт контроллер
func NewWellKnownController(cfg *config.Config) *WellKnownController {
	return &WellKnownController{cfg: cfg}
}

// assetLinksEntry — один элемент массива assetlinks.json (Digital Asset Links)
type assetLinksEntry struct {
	Relation []string    `json:"relation"`
	Target   targetAsset `json:"target"`
}

type targetAsset struct {
	Namespace               string   `json:"namespace"`
	PackageName             string   `json:"package_name"`
	SHA256CertFingerprints  []string `json:"sha256_cert_fingerprints"`
}

// AssetLinks отдаёт /.well-known/assetlinks.json для Android App Links
func (w *WellKnownController) AssetLinks(c *gin.Context) {
	fingerprints := w.cfg.AndroidSHA256Fingerprints
	if len(fingerprints) == 0 {
		fingerprints = []string{"F7:34:EE:03:5C:83:AA:B7:EF:44:43:67:95:28:9B:D0:16:99:0F:E5:52:B8:0F:98:E5:12:76:F2:33:E2"}
	}
	packageName := w.cfg.AndroidPackageName
	if packageName == "" {
		packageName = "com.kliro.app"
	}
	body := []assetLinksEntry{
		{
			Relation: []string{
				"delegate_permission/common.handle_all_urls",
				"delegate_permission/common.get_login_creds",
			},
			Target: targetAsset{
				Namespace:              "android_app",
				PackageName:            packageName,
				SHA256CertFingerprints: fingerprints,
			},
		},
	}
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, body)
}

// appleAppSiteAssociation — формат AASA для iOS Universal Links
type appleAppSiteAssociation struct {
	AppLinks appLinksDetail `json:"applinks"`
}

type appLinksDetail struct {
	Apps    []string        `json:"apps"`
	Details []aasaDetailEntry `json:"details"`
}

type aasaDetailEntry struct {
	AppID string   `json:"appID"`
	Paths []string `json:"paths"`
}

// AppleAppSiteAssociation отдаёт /.well-known/apple-app-site-association для iOS
func (w *WellKnownController) AppleAppSiteAssociation(c *gin.Context) {
	teamID := w.cfg.AppleTeamID
	bundleID := w.cfg.AppleBundleID
	if bundleID == "" {
		bundleID = "com.kliro.app"
	}
	aasa := appleAppSiteAssociation{
		AppLinks: appLinksDetail{
			Apps: []string{},
			Details: []aasaDetailEntry{},
		},
	}
	if teamID != "" {
		aasa.AppLinks.Details = []aasaDetailEntry{
			{
				AppID: teamID + "." + bundleID,
				Paths: []string{"*"},
			},
		}
	}
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, aasa)
}

package crypto

import (
	"net/http"

	"ymessage/internal/database"
	"ymessage/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PrekeyUploadReq struct {
	IdentityKey    string   `json:"identity_key" binding:"required"`
	SignedPrekey   string   `json:"signed_prekey" binding:"required"`
	SignedPrekeyID uint32   `json:"signed_prekey_id" binding:"required"`
	Signature      string   `json:"signature" binding:"required"`
	OneTimePrekeys []OneTimeKeyDTO `json:"one_time_prekeys"`
}

type OneTimeKeyDTO struct {
	KeyID  uint32 `json:"key_id" binding:"required"`
	KeyVal string `json:"key_val" binding:"required"`
}

// UploadPrekeyBundle uploads or updates identity/signed prekeys and adds new one-time prekeys
func UploadPrekeyBundle(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req PrekeyUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Create or update bundle
		var bundle models.PrekeyBundle
		err := tx.Where("user_id = ?", userID).First(&bundle).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				bundle = models.PrekeyBundle{
					UserID: userID.(uuid.UUID),
				}
			} else {
				return err
			}
		}

		bundle.IdentityKey = req.IdentityKey
		bundle.SignedPrekey = req.SignedPrekey
		bundle.SignedPrekeyID = req.SignedPrekeyID
		bundle.Signature = req.Signature

		if err := tx.Save(&bundle).Error; err != nil {
			return err
		}

		// Insert one-time prekeys
		if len(req.OneTimePrekeys) > 0 {
			var otkeys []models.OneTimePrekey
			for _, ot := range req.OneTimePrekeys {
				otkeys = append(otkeys, models.OneTimePrekey{
					UserID: userID.(uuid.UUID),
					KeyID:  ot.KeyID,
					KeyVal: ot.KeyVal,
					Used:   false,
				})
			}
			// Batch insert
			if err := tx.Create(&otkeys).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload prekeys: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Prekey bundle uploaded successfully"})
}

// GetPrekeyBundle fetches E2EE bundle for a recipient and consumes a One-Time Prekey
func GetPrekeyBundle(c *gin.Context) {
	recipientIDStr := c.Param("userId")
	recipientID, err := uuid.Parse(recipientIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipient user ID"})
		return
	}

	var bundle models.PrekeyBundle
	if err := database.DB.Where("user_id = ?", recipientID).First(&bundle).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "E2EE keys not set up for this user"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error fetching prekey bundle"})
		}
		return
	}

	// Fetch and consume one-time prekey in a transaction
	var otkey models.OneTimePrekey
	hasOtkey := false

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("user_id = ? AND used = ?", recipientID, false).Order("id ASC").First(&otkey).Error
		if err == nil {
			otkey.Used = true
			if err := tx.Save(&otkey).Error; err != nil {
				return err
			}
			hasOtkey = true
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction error fetching one-time prekey"})
		return
	}

	resp := gin.H{
		"user_id":          bundle.UserID,
		"identity_key":     bundle.IdentityKey,
		"signed_prekey":    bundle.SignedPrekey,
		"signed_prekey_id": bundle.SignedPrekeyID,
		"signature":        bundle.Signature,
	}

	if hasOtkey {
		resp["one_time_prekey"] = gin.H{
			"key_id":  otkey.KeyID,
			"key_val": otkey.KeyVal,
		}
	} else {
		resp["one_time_prekey"] = nil
	}

	c.JSON(http.StatusOK, resp)
}

// GetPrekeyStatus gets count of remaining unused one-time prekeys
func GetPrekeyStatus(c *gin.Context) {
	userID, _ := c.Get("userID")
	var count int64
	database.DB.Model(&models.OneTimePrekey{}).Where("user_id = ? AND used = ?", userID, false).Count(&count)
	c.JSON(http.StatusOK, gin.H{
		"remaining_one_time_prekeys": count,
	})
}

package repository

import (
	"crypto/sha1"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/traPtitech/traQ/model"
	"github.com/traPtitech/traQ/service/ogp"
	"reflect"
)

func getURLHash(url string) (string, error) {
	hash := sha1.New()
	_, _ = hash.Write([]byte(url))
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// CreateOgpCache implements OgpRepository interface.
func (repo *GormRepository) CreateOgpCache(url string, content *model.Ogp) (c *model.OgpCache, err error) {
	urlHash, err := getURLHash(url)
	if err != nil {
		return nil, err
	}

	ogpCache := &model.OgpCache{
		URL:       url,
		URLHash:   urlHash,
		Content:   model.Ogp{},
		Valid:     content != nil,
		ExpiresAt: ogp.GetCacheExpireDate(),
	}

	if content != nil {
		ogpCache.Content = *content
	}

	err = repo.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(ogpCache).Error
	})
	if err != nil {
		return nil, err
	}
	return ogpCache, nil
}

// UpdateOgpCache implements OgpRepository interface.
func (repo *GormRepository) UpdateOgpCache(url string, content *model.Ogp) error {
	urlHash, err := getURLHash(url)
	if err != nil {
		return err
	}

	changes := map[string]interface{}{}
	return repo.db.Transaction(func(tx *gorm.DB) error {
		var c model.OgpCache
		if err := tx.First(&c, &model.OgpCache{URL: url, URLHash: urlHash}).Error; err != nil {
			return convertError(err)
		}

		if content == nil {
			changes["valid"] = false
			changes["content"] = model.Ogp{}
			changes["expires_at"] = ogp.GetCacheExpireDate()
			return tx.Model(&c).Updates(changes).Error
		}
		if !reflect.DeepEqual(c.Content, content) {
			changes["valid"] = true
			changes["content"] = content
			changes["expires_at"] = ogp.GetCacheExpireDate()
			return tx.Model(&c).Updates(changes).Error
		}
		return nil
	})
}

// GetOgpCache implements OgpRepository interface.
func (repo *GormRepository) GetOgpCache(url string) (c *model.OgpCache, err error) {
	urlHash, err := getURLHash(url)
	if err != nil {
		return nil, err
	}

	c = &model.OgpCache{}
	if err = repo.db.Take(c, &model.OgpCache{URL: url, URLHash: urlHash}).Error; err != nil {
		return nil, convertError(err)
	}
	return c, nil
}

// DeleteOgpCache implements OgpRepository interface.
func (repo *GormRepository) DeleteOgpCache(url string) error {
	c, err := repo.GetOgpCache(url)
	if err != nil {
		return err
	}
	result := repo.db.Delete(c)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

package model

import (
	"github.com/jinzhu/gorm"
	"github.com/satori/go.uuid"
	"github.com/traPtitech/traQ/utils/validator"
	"time"
)

// Stamp スタンプ構造体
type Stamp struct {
	ID        string     `gorm:"type:char(36);primary_key" json:"id"`
	Name      string     `gorm:"size:32;unique"            json:"name"      validate:"name,required"`
	CreatorID string     `gorm:"type:char(36)"             json:"creatorId" validate:"uuid,required"`
	FileID    string     `gorm:"type:char(36)"             json:"fileId"    validate:"uuid,required"`
	CreatedAt time.Time  `gorm:"precision:6"               json:"createdAt"`
	UpdatedAt time.Time  `gorm:"precision:6"               json:"updatedAt"`
	DeletedAt *time.Time `gorm:"precision:6"`
}

// TableName スタンプテーブル名を取得します
func (*Stamp) TableName() string {
	return "stamps"
}

// BeforeCreate db.Create時に自動的に呼ばれます
func (s *Stamp) BeforeCreate(scope *gorm.Scope) error {
	s.ID = CreateUUID()
	return s.Validate()
}

// Validate 構造体を検証します
func (s *Stamp) Validate() error {
	return validator.ValidateStruct(s)
}

// UpdateStamp スタンプを更新します
func UpdateStamp(stampId uuid.UUID, s Stamp) error {
	s.ID = ""
	s.CreatedAt = time.Time{}
	s.UpdatedAt = time.Time{}
	s.DeletedAt = nil
	if err := validator.ValidateVar(s.Name, "name"); err != nil {
		return err
	}
	if err := validator.ValidateVar(s.CreatorID, "uuid,omitempty"); err != nil {
		return err
	}
	if err := validator.ValidateVar(s.FileID, "uuid,omitempty"); err != nil {
		return err
	}

	return db.Where(Stamp{ID: stampId.String()}).Updates(&s).Error
}

// CreateStamp スタンプを作成します
func CreateStamp(name, fileID, userID string) (*Stamp, error) {
	stamp := &Stamp{
		Name:      name,
		CreatorID: userID,
		FileID:    fileID,
	}

	if err := db.Create(stamp).Error; err != nil {
		return nil, err
	}

	return stamp, nil
}

// GetStamp 指定したIDのスタンプを取得します
func GetStamp(id string) (*Stamp, error) {
	s := &Stamp{}
	if err := db.Where(Stamp{ID: id}).Take(s).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s, nil
}

// DeleteStamp 指定したIDのスタンプを削除します
func DeleteStamp(id string) error {
	return db.Delete(Stamp{ID: id}).Error
}

// GetAllStamps 全てのスタンプを取得します
func GetAllStamps() (stamps []Stamp, err error) {
	err = db.Find(&stamps).Error
	return
}

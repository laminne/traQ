package repository

import (
	"github.com/gofrs/uuid"
	"github.com/leandro-lugaresi/hub"
	"github.com/traPtitech/traQ/event"
	"github.com/traPtitech/traQ/model"
	"github.com/traPtitech/traQ/utils/gormutil"
	"unicode/utf8"
)

// GetTagByID implements TagRepository interface.
func (repo *GormRepository) GetTagByID(id uuid.UUID) (*model.Tag, error) {
	if id == uuid.Nil {
		return nil, ErrNotFound
	}
	tag := &model.Tag{}
	if err := repo.db.Take(tag, &model.Tag{ID: id}).Error; err != nil {
		return nil, convertError(err)
	}
	return tag, nil
}

// GetOrCreateTag implements TagRepository interface.
func (repo *GormRepository) GetOrCreateTag(name string) (*model.Tag, error) {
	if len(name) == 0 {
		return nil, ErrNotFound
	}
	if utf8.RuneCountInString(name) > 30 {
		return nil, ArgError("name", "tag must be non-empty and shorter than 31 characters")
	}
	tag := &model.Tag{}
	err := repo.db.
		Where(&model.Tag{Name: name}).
		Attrs(&model.Tag{ID: uuid.Must(uuid.NewV4())}).
		FirstOrCreate(tag).
		Error
	return tag, err
}

// AddUserTag implements TagRepository interface.
func (repo *GormRepository) AddUserTag(userID, tagID uuid.UUID) error {
	if userID == uuid.Nil || tagID == uuid.Nil {
		return ErrNilID
	}
	ut := &model.UsersTag{
		UserID: userID,
		TagID:  tagID,
	}
	// TODO タグの存在確認
	if err := repo.db.Create(ut).Error; err != nil {
		if gormutil.IsMySQLDuplicatedRecordErr(err) {
			return ErrAlreadyExists
		}
		return err
	}
	repo.hub.Publish(hub.Message{
		Name: event.UserTagAdded,
		Fields: hub.Fields{
			"user_id": userID,
			"tag_id":  tagID,
		},
	})
	return nil
}

// ChangeUserTagLock implements TagRepository interface.
func (repo *GormRepository) ChangeUserTagLock(userID, tagID uuid.UUID, locked bool) error {
	if userID == uuid.Nil || tagID == uuid.Nil {
		return ErrNilID
	}

	result := repo.db.Model(&model.UsersTag{UserID: userID, TagID: tagID}).Update("is_locked", locked)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	repo.hub.Publish(hub.Message{
		Name: event.UserTagUpdated,
		Fields: hub.Fields{
			"user_id": userID,
			"tag_id":  tagID,
		},
	})
	return nil
}

// DeleteUserTag implements TagRepository interface.
func (repo *GormRepository) DeleteUserTag(userID, tagID uuid.UUID) error {
	if userID == uuid.Nil || tagID == uuid.Nil {
		return ErrNilID
	}
	result := repo.db.Delete(&model.UsersTag{UserID: userID, TagID: tagID})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		repo.hub.Publish(hub.Message{
			Name: event.UserTagRemoved,
			Fields: hub.Fields{
				"user_id": userID,
				"tag_id":  tagID,
			},
		})
	}
	return nil
}

// GetUserTag implements TagRepository interface.
func (repo *GormRepository) GetUserTag(userID, tagID uuid.UUID) (model.UserTag, error) {
	if userID == uuid.Nil || tagID == uuid.Nil {
		return nil, ErrNotFound
	}
	ut := &model.UsersTag{}
	if err := repo.db.Preload("Tag").Take(ut, &model.UsersTag{UserID: userID, TagID: tagID}).Error; err != nil {
		return nil, convertError(err)
	}
	return ut, nil
}

// GetUserTagsByUserID implements TagRepository interface.
func (repo *GormRepository) GetUserTagsByUserID(userID uuid.UUID) (tags []model.UserTag, err error) {
	var tmp []*model.UsersTag
	if userID == uuid.Nil {
		return tags, nil
	}
	err = repo.db.
		Preload("Tag").
		Where(&model.UsersTag{UserID: userID}).
		Order("created_at").
		Find(&tmp).
		Error
	tags = make([]model.UserTag, len(tmp))
	for i, tag := range tmp {
		tags[i] = tag
	}
	return tags, err
}

// GetUserIDsByTagID implements TagRepository interface.
func (repo *GormRepository) GetUserIDsByTagID(tagID uuid.UUID) (arr []uuid.UUID, err error) {
	arr = make([]uuid.UUID, 0)
	if tagID == uuid.Nil {
		return arr, nil
	}
	err = repo.db.
		Model(&model.UsersTag{}).
		Where(&model.UsersTag{TagID: tagID}).
		Pluck("user_id", &arr).
		Error
	return arr, err
}

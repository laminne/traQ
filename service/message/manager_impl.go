package message

import (
	"context"
	"fmt"
	"github.com/bluele/gcache"
	"github.com/gofrs/uuid"
	"github.com/traPtitech/traQ/model"
	"github.com/traPtitech/traQ/repository"
	"github.com/traPtitech/traQ/service/channel"
	"go.uber.org/zap"
	"sync"
	"time"
)

var cacheTTL = time.Minute

type manager struct {
	CM channel.Manager
	R  repository.Repository
	L  *zap.Logger
	P  sync.WaitGroup

	cache gcache.Cache
}

func NewMessageManager(repo repository.Repository, cm channel.Manager, logger *zap.Logger) (Manager, error) {
	return &manager{
		CM: cm,
		R:  repo,
		L:  logger.Named("message_manager"),
		cache: gcache.
			New(200).
			ARC().
			LoaderExpireFunc(func(key interface{}) (interface{}, *time.Duration, error) {
				m, err := repo.GetMessageByID(key.(uuid.UUID))
				if err != nil {
					if err == repository.ErrNotFound {
						return nil, nil, ErrNotFound
					}
					return nil, nil, fmt.Errorf("failed to GetMessageByID: %w", err)
				}
				return &message{Model: m}, &cacheTTL, nil
			}).
			Build(),
	}, nil
}

func (m *manager) Get(id uuid.UUID) (Message, error) {
	return m.get(id)
}

func (m *manager) get(id uuid.UUID) (*message, error) {
	if id == uuid.Nil {
		return nil, ErrNotFound
	}

	// メモリキャッシュから取得。キャッシュに無い場合はキャッシュのLoaderFuncで自動取得し、キャッシュに追加
	mI, err := m.cache.Get(id)
	if err != nil {
		return nil, err
	}
	return mI.(*message), nil
}

func (m *manager) GetTimeline(query TimelineQuery) (Timeline, error) {
	q := repository.MessagesQuery{
		User:                     query.User,
		Channel:                  query.Channel,
		ChannelsSubscribedByUser: query.ChannelsSubscribedByUser,
		Since:                    query.Since,
		Until:                    query.Until,
		Inclusive:                query.Inclusive,
		Limit:                    query.Limit,
		Offset:                   query.Offset,
		Asc:                      query.Asc,
		ExcludeDMs:               query.ExcludeDMs,
		DisablePreload:           query.DisablePreload,
	}
	messages, more, err := m.R.GetMessages(q)
	if err != nil {
		return nil, fmt.Errorf("failed to GetMessages: %w", err)
	}

	return &timeline{
		query:       query,
		records:     messages,
		more:        more,
		preloaded:   !q.DisablePreload,
		retrievedAt: time.Now(),
		man:         m,
	}, nil
}

func (m *manager) CreateDM(from, to uuid.UUID, content string) (Message, error) {
	// DMチャンネルを取得
	ch, err := m.CM.GetDMChannel(from, to)
	if err != nil {
		return nil, err
	}

	return m.create(ch.ID, from, content)
}

func (m *manager) Create(channelID, userID uuid.UUID, content string) (Message, error) {
	// チャンネルがアーカイブされているかどうか確認
	if m.CM.IsPublicChannel(channelID) && m.CM.PublicChannelTree().IsArchivedChannel(channelID) {
		return nil, ErrChannelArchived
	}

	return m.create(channelID, userID, content)
}

func (m *manager) create(channelID, userID uuid.UUID, content string) (Message, error) {
	// 作成
	msg, err := m.R.CreateMessage(userID, channelID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to CreateMessage: %w", err)
	}

	// メモリにキャッシュ
	wrapped := &message{Model: msg}
	_ = m.cache.SetWithExpire(msg.ID, wrapped, cacheTTL)
	return wrapped, nil
}

func (m *manager) Edit(id uuid.UUID, content string) error {
	// メッセージ取得
	msg, err := m.Get(id)
	if err != nil {
		return err
	}

	// チャンネルがアーカイブされているかどうか確認
	if m.CM.IsPublicChannel(msg.GetChannelID()) && m.CM.PublicChannelTree().IsArchivedChannel(msg.GetChannelID()) {
		return ErrChannelArchived
	}

	// 更新
	if err := m.R.UpdateMessage(id, content); err != nil {
		switch err {
		case repository.ErrNotFound:
			return ErrNotFound
		default:
			return fmt.Errorf("failed to UpdateMessage: %w", err)
		}
	}
	m.cache.Remove(id)

	return nil
}

func (m *manager) Delete(id uuid.UUID) error {
	// メッセージ取得
	msg, err := m.Get(id)
	if err != nil {
		return err
	}

	// チャンネルがアーカイブされているかどうか確認
	if m.CM.IsPublicChannel(msg.GetChannelID()) && m.CM.PublicChannelTree().IsArchivedChannel(msg.GetChannelID()) {
		return ErrChannelArchived
	}

	// 削除
	if err := m.R.DeleteMessage(id); err != nil {
		switch err {
		case repository.ErrNotFound:
			return ErrNotFound
		default:
			return fmt.Errorf("failed to DeleteMessage: %w", err)
		}
	}
	m.cache.Remove(id)

	return nil
}

func (m *manager) Pin(id uuid.UUID, userID uuid.UUID) (*model.Pin, error) {
	// メッセージ取得
	msg, err := m.Get(id)
	if err != nil {
		return nil, err
	}

	// すでにピンされているか
	if msg.GetPin() != nil {
		return nil, ErrAlreadyExists
	}

	// チャンネルがアーカイブされているかどうか確認
	if m.CM.IsPublicChannel(msg.GetChannelID()) && m.CM.PublicChannelTree().IsArchivedChannel(msg.GetChannelID()) {
		return nil, ErrChannelArchived
	}

	// ピン
	pin, err := m.R.PinMessage(id, userID)
	if err != nil {
		switch err {
		case repository.ErrNotFound:
			return nil, ErrNotFound
		case repository.ErrAlreadyExists:
			return nil, ErrAlreadyExists
		default:
			return nil, fmt.Errorf("failed to PinMessage: %w", err)
		}
	}
	m.cache.Remove(id)

	// ロギング
	m.recordChannelEvent(pin.Message.ChannelID, model.ChannelEventPinAdded, model.ChannelEventDetail{
		"userId":    userID,
		"messageId": pin.MessageID,
	}, pin.CreatedAt)
	return pin, nil
}

func (m *manager) Unpin(id uuid.UUID, userID uuid.UUID) error {
	// メッセージ取得
	msg, err := m.Get(id)
	if err != nil {
		return err
	}

	// ピンがあるかどうか
	if msg.GetPin() == nil {
		return ErrNotFound
	}

	// チャンネルがアーカイブされているかどうか確認
	if m.CM.IsPublicChannel(msg.GetChannelID()) && m.CM.PublicChannelTree().IsArchivedChannel(msg.GetChannelID()) {
		return ErrChannelArchived
	}

	// ピン外し
	pin, err := m.R.UnpinMessage(id)
	if err != nil {
		switch err {
		case repository.ErrNotFound:
			return ErrNotFound
		default:
			return fmt.Errorf("failed to UnpinMessage: %w", err)
		}
	}
	m.cache.Remove(id)

	// ロギング
	m.recordChannelEvent(pin.Message.ChannelID, model.ChannelEventPinRemoved, model.ChannelEventDetail{
		"userId":    userID,
		"messageId": pin.MessageID,
	}, time.Now())
	return nil
}

func (m *manager) AddStamps(id, stampID, userID uuid.UUID, n int) (*model.MessageStamp, error) {
	// メッセージ取得
	msg, err := m.get(id)
	if err != nil {
		return nil, err
	}

	// チャンネルがアーカイブされているかどうか確認
	if m.CM.IsPublicChannel(msg.GetChannelID()) && m.CM.PublicChannelTree().IsArchivedChannel(msg.GetChannelID()) {
		return nil, ErrChannelArchived
	}

	// スタンプを押す
	ms, err := m.R.AddStampToMessage(id, stampID, userID, n)
	if err != nil {
		return nil, fmt.Errorf("failed to AddStampToMessage: %w", err)
	}

	// キャッシュ更新
	msg.UpdateStamp(ms)

	return ms, nil
}

func (m *manager) RemoveStamps(id, stampID, userID uuid.UUID) error {
	// メッセージ取得
	msg, err := m.get(id)
	if err != nil {
		return err
	}

	// チャンネルがアーカイブされているかどうか確認
	if m.CM.IsPublicChannel(msg.GetChannelID()) && m.CM.PublicChannelTree().IsArchivedChannel(msg.GetChannelID()) {
		return ErrChannelArchived
	}

	// スタンプを消す
	if err := m.R.RemoveStampFromMessage(id, stampID, userID); err != nil {
		return fmt.Errorf("failed to RemoveStampFromMessage: %w", err)
	}

	// キャッシュ更新
	msg.RemoveStamp(stampID, userID)

	return nil
}

func (m *manager) Wait(ctx context.Context) error {
	m.P.Wait()
	return nil
}

func (m *manager) recordChannelEvent(channelID uuid.UUID, eventType model.ChannelEventType, detail model.ChannelEventDetail, datetime time.Time) {
	m.P.Add(1)
	go func() {
		defer m.P.Done()

		err := m.R.RecordChannelEvent(channelID, eventType, detail, datetime)
		if err != nil {
			m.L.Warn("failed to record channel event", zap.Error(err), zap.Stringer("channelID", channelID), zap.Stringer("type", eventType), zap.Any("detail", detail), zap.Time("datetime", datetime))
		}
	}()
}

package v1

import (
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/labstack/echo/v4"
	"github.com/leandro-lugaresi/hub"
	"github.com/traPtitech/traQ/bot"
	"github.com/traPtitech/traQ/event"
	"github.com/traPtitech/traQ/model"
	"github.com/traPtitech/traQ/rbac/permission"
	"github.com/traPtitech/traQ/rbac/role"
	"github.com/traPtitech/traQ/repository"
	"github.com/traPtitech/traQ/router/consts"
	"github.com/traPtitech/traQ/router/extension/herror"
	"gopkg.in/guregu/null.v3"
	"net/http"
)

// GetBots GET /bots
func (h *Handlers) GetBots(c echo.Context) error {
	user := getRequestUser(c)

	var (
		list []*model.Bot
		err  error
		q    repository.BotsQuery
	)
	if c.QueryParam("all") == "1" && h.RBAC.IsGranted(user.Role, permission.AccessOthersBot) {
		list, err = h.Repo.GetBots(q)
	} else {
		list, err = h.Repo.GetBots(q.CreatedBy(user.ID))
	}
	if err != nil {
		return herror.InternalServerError(err)
	}

	return c.JSON(http.StatusOK, formatBots(list))
}

// PostBots POST /bots
func (h *Handlers) PostBots(c echo.Context) error {
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		WebhookURL  string `json:"webhookUrl"`
	}
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	b, err := h.Repo.CreateBot(req.Name, req.DisplayName, req.Description, getRequestUserID(c), req.WebhookURL)
	if err != nil {
		switch {
		case repository.IsArgError(err):
			return herror.BadRequest(err)
		case err == repository.ErrAlreadyExists:
			return herror.Conflict("this name has already been used")
		default:
			return herror.InternalServerError(err)
		}
	}

	t, err := h.Repo.GetTokenByID(b.AccessTokenID)
	if err != nil {
		return herror.InternalServerError(err)
	}

	return c.JSON(http.StatusCreated, formatBotDetail(b, t))
}

// GetBot GET /bots/:botID
func (h *Handlers) GetBot(c echo.Context) error {
	b := getBotFromContext(c)
	return c.JSON(http.StatusOK, formatBot(b))
}

// PatchBot PATCH /bots/:botID
func (h *Handlers) PatchBot(c echo.Context) error {
	b := getBotFromContext(c)

	var req struct {
		DisplayName null.String   `json:"displayName"`
		Description null.String   `json:"description"`
		WebhookURL  null.String   `json:"webhookUrl"`
		Privileged  null.Bool     `json:"privileged"`
		CreatorID   uuid.NullUUID `json:"creatorId"`
	}
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	if req.Privileged.Valid && getRequestUser(c).Role != role.Admin {
		return herror.Forbidden("you are not permitted to set privileged flag to bots")
	}

	args := repository.UpdateBotArgs{
		DisplayName: req.DisplayName,
		Description: req.Description,
		WebhookURL:  req.WebhookURL,
		Privileged:  req.Privileged,
		CreatorID:   req.CreatorID,
	}

	if err := h.Repo.UpdateBot(b.ID, args); err != nil {
		switch {
		case repository.IsArgError(err):
			return herror.BadRequest(err)
		default:
			return herror.InternalServerError(err)
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// DeleteBot DELETE /bots/:botID
func (h *Handlers) DeleteBot(c echo.Context) error {
	b := getBotFromContext(c)

	if err := h.Repo.DeleteBot(b.ID); err != nil {
		return herror.InternalServerError(err)
	}

	return c.NoContent(http.StatusNoContent)
}

// GetBotDetail GET /bots/:botID/detail
func (h *Handlers) GetBotDetail(c echo.Context) error {
	b := getBotFromContext(c)

	t, err := h.Repo.GetTokenByID(b.AccessTokenID)
	if err != nil {
		switch err {
		case repository.ErrNotFound:
			return herror.HTTPError(http.StatusInternalServerError, "This bot's Access Token has been revoked unexpectedly. Please inform admin about this error.")
		default:
			return herror.InternalServerError(err)
		}
	}

	return c.JSON(http.StatusOK, formatBotDetail(b, t))
}

// PutBotEvents PUT /bots/:botID/events
func (h *Handlers) PutBotEvents(c echo.Context) error {
	b := getBotFromContext(c)

	var req struct {
		Events []string `json:"events"`
	}
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	events := model.BotEvents{}
	for _, v := range req.Events {
		if !bot.IsEvent(v) {
			return herror.BadRequest(fmt.Sprintf("invalid event: %s", v))
		}
		events[model.BotEvent(v)] = true
	}

	if err := h.Repo.SetSubscribeEventsToBot(b.ID, events); err != nil {
		return herror.InternalServerError(err)
	}

	return c.NoContent(http.StatusNoContent)
}

// GetBotIcon GET /bots/:botID/icon
func (h *Handlers) GetBotIcon(c echo.Context) error {
	b := getBotFromContext(c)

	// ユーザー取得
	user, err := h.Repo.GetUser(b.BotUserID)
	if err != nil {
		return herror.InternalServerError(err)
	}

	return h.getUserIcon(c, user)
}

// PutBotIcon PUT /bots/:botID/icon
func (h *Handlers) PutBotIcon(c echo.Context) error {
	b := getBotFromContext(c)
	return h.putUserIcon(c, b.BotUserID)
}

// PutBotState PUT /bots/:botID/state
func (h *Handlers) PutBotState(c echo.Context) error {
	b := getBotFromContext(c)

	var req struct {
		State string `json:"state"`
	}
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	switch req.State {
	case "active":
		h.Hub.Publish(hub.Message{
			Name: event.BotPingRequest,
			Fields: hub.Fields{
				"bot_id": b.ID,
				"bot":    b,
			},
		})
		return c.NoContent(http.StatusAccepted)
	case "inactive":
		if err := h.Repo.ChangeBotState(b.ID, model.BotInactive); err != nil {
			return herror.InternalServerError(err)
		}
		return c.NoContent(http.StatusNoContent)
	default:
		return herror.BadRequest("invalid state")
	}
}

// PostBotReissueTokens POST /bots/:botID/reissue
func (h *Handlers) PostBotReissueTokens(c echo.Context) error {
	b := getBotFromContext(c)

	b, err := h.Repo.ReissueBotTokens(b.ID)
	if err != nil {
		return herror.InternalServerError(err)
	}

	t, err := h.Repo.GetTokenByID(b.AccessTokenID)
	if err != nil {
		return herror.InternalServerError(err)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"verificationCode": b.VerificationToken,
		"accessToken":      t.AccessToken,
		"botCode":          b.BotCode,
	})
}

// GetBotJoinChannels GET /bots/:botID/channels
func (h *Handlers) GetBotJoinChannels(c echo.Context) error {
	b := getBotFromContext(c)

	ids, err := h.Repo.GetParticipatingChannelIDsByBot(b.ID)
	if err != nil {
		return herror.InternalServerError(err)
	}

	return c.JSON(http.StatusOK, ids)
}

// GetBotEventLogs GET /bots/:botID/events/logs
func (h *Handlers) GetBotEventLogs(c echo.Context) error {
	b := getBotFromContext(c)

	var req struct {
		Limit  int `query:"limit"`
		Offset int `query:"offset"`
	}
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 50
	}

	logs, err := h.Repo.GetBotEventLogs(b.ID, req.Limit, req.Offset)
	if err != nil {
		return herror.InternalServerError(err)
	}

	return c.JSON(http.StatusOK, logs)
}

// GetChannelBots GET /channels/:channelID/bots
func (h *Handlers) GetChannelBots(c echo.Context) error {
	channelID := getRequestParamAsUUID(c, consts.ParamChannelID)

	bots, err := h.Repo.GetBots(repository.BotsQuery{}.CMemberOf(channelID))
	if err != nil {
		return herror.InternalServerError(err)
	}

	type response struct {
		BotID     uuid.UUID `json:"botId"`
		BotUserID uuid.UUID `json:"botUserId"`
	}
	res := make([]response, len(bots))
	for i, v := range bots {
		res[i] = response{
			BotID:     v.ID,
			BotUserID: v.BotUserID,
		}
	}

	return c.JSON(http.StatusOK, res)
}

// PostChannelBots POST /channels/:channelID/bots
func (h *Handlers) PostChannelBots(c echo.Context) error {
	ch := getChannelFromContext(c)
	if !ch.IsPublic {
		return echo.NewHTTPError(http.StatusForbidden)
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	b, err := h.Repo.GetBotByCode(req.Code)
	if err != nil {
		switch err {
		case repository.ErrNotFound:
			return herror.BadRequest("invalid bot code")
		default:
			return herror.InternalServerError(err)
		}
	}

	if err := h.Repo.AddBotToChannel(b.ID, ch.ID); err != nil {
		return herror.InternalServerError(err)
	}

	return c.JSON(http.StatusOK, echo.Map{"botId": b.ID})
}

// DeleteChannelBot DELETE /channels/:channelID/bots/:botID
func (h *Handlers) DeleteChannelBot(c echo.Context) error {
	channelID := getRequestParamAsUUID(c, consts.ParamChannelID)
	botID := getRequestParamAsUUID(c, consts.ParamBotID)

	if err := h.Repo.RemoveBotFromChannel(botID, channelID); err != nil {
		return herror.InternalServerError(err)
	}

	return c.NoContent(http.StatusNoContent)
}

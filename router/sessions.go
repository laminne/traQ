package router

import (
	"github.com/labstack/echo"
	"github.com/traPtitech/traQ/sessions"
	"net/http"
	"time"
)

// GetMySessions GET /users/me/sessions
func GetMySessions(c echo.Context) error {
	userID := getRequestUserID(c)

	ses, err := sessions.GetByUserID(userID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	type response struct {
		ID            string    `json:"id"`
		LastIP        string    `json:"lastIP"`
		LastUserAgent string    `json:"lastUserAgent"`
		LastAccess    time.Time `json:"lastAccess"`
		CreatedAt     time.Time `json:"createdAt"`
	}

	res := make([]response, len(ses))
	for k, v := range ses {
		referenceID, created, lastAccess, lastIP, lastUserAgent := v.GetSessionInfo()
		res[k] = response{
			ID:            referenceID.String(),
			LastIP:        lastIP,
			LastUserAgent: lastUserAgent,
			LastAccess:    lastAccess,
			CreatedAt:     created,
		}
	}

	return c.JSON(http.StatusOK, res)
}

// DeleteAllMySessions DELETE /users/me/sessions
func DeleteAllMySessions(c echo.Context) error {
	userID := getRequestUserID(c)

	err := sessions.DestroyByUserID(userID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}

// DeleteMySession DELETE /users/me/sessions/:referenceID
func DeleteMySession(c echo.Context) error {
	userID := getRequestUserID(c)
	referenceID := getRequestParamAsUUID(c, paramReferenceID)

	err := sessions.DestroyByReferenceID(userID, referenceID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}
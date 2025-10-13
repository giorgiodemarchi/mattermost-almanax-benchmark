// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

const (
	sessionsCleanupDelay = 100 * time.Millisecond
)

type SqlSessionStore struct {
	*SqlStore

	sessionSelectQuery sq.SelectBuilder
}

func newSqlSessionStore(sqlStore *SqlStore) store.SessionStore {
	s := &SqlSessionStore{
		SqlStore: sqlStore,
	}

	s.sessionSelectQuery = s.getQueryBuilder().
		Select("Id", "Token", "CreateAt", "ExpiresAt", "LastActivityAt", "UserId", "DeviceId", "Roles", "IsOAuth", "ExpiredNotify", "Props").
		From("Sessions")

	return s
}

func (me SqlSessionStore) Save(rctx request.CTX, session *model.Session) (*model.Session, error) {
	if session.Id != "" {
		return nil, store.NewErrInvalidInput("Session", "id", session.Id)
	}

	session.PreSave()

	if err := session.IsValid(); err != nil {
		return nil, err
	}
	jsonProps, err := json.Marshal(session.Props)
	if err != nil {
		return nil, errors.Wrap(err, "failed marshalling session props")
	}

	if me.IsBinaryParamEnabled() {
		jsonProps = AppendBinaryFlag(jsonProps)
	}

	query, args, err := me.getQueryBuilder().
		Insert("Sessions").
		Columns("Id", "Token", "CreateAt", "ExpiresAt", "LastActivityAt", "UserId", "DeviceId", "Roles", "IsOAuth", "ExpiredNotify", "Props").
		Values(session.Id, session.Token, session.CreateAt, session.ExpiresAt, session.LastActivityAt, session.UserId, session.DeviceId, session.Roles, session.IsOAuth, session.ExpiredNotify, jsonProps).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "sessions_tosql")
	}
	if _, err = me.GetMaster().Exec(query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to save Session with id=%s", session.Id)
	}

	teamMembers, err := me.Team().GetTeamsForUser(rctx, session.UserId, "", true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find TeamMembers for Session with userId=%s", session.UserId)
	}

	session.TeamMembers = make([]*model.TeamMember, 0, len(teamMembers))
	for _, tm := range teamMembers {
		if tm.DeleteAt == 0 {
			session.TeamMembers = append(session.TeamMembers, tm)
		}
	}

	return session, nil
}

func (me SqlSessionStore) Get(rctx request.CTX, sessionIdOrToken string) (*model.Session, error) {
	sessions := []*model.Session{}

	query := me.sessionSelectQuery.
		Where(sq.Or{
			sq.Eq{"Token": sessionIdOrToken},
			sq.Eq{"Id": sessionIdOrToken},
		}).
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "session_get_tosql")
	}

	err = me.DBXFromContext(rctx.Context()).Select(&sessions, sql, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Sessions with sessionIdOrToken=%s", sessionIdOrToken)
	}
	if len(sessions) == 0 {
		return nil, store.NewErrNotFound("Session", fmt.Sprintf("sessionIdOrToken=%s", sessionIdOrToken))
	}
	session := sessions[0]

	tempMembers, err := me.Team().GetTeamsForUser(
		RequestContextWithMaster(rctx),
		session.UserId, "", true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find TeamMembers for Session with userId=%s", session.UserId)
	}
	sessions[0].TeamMembers = make([]*model.TeamMember, 0, len(tempMembers))
	for _, tm := range tempMembers {
		if tm.DeleteAt == 0 {
			sessions[0].TeamMembers = append(sessions[0].TeamMembers, tm)
		}
	}
	return session, nil
}

func (me SqlSessionStore) GetSessions(rctx request.CTX, userId string) ([]*model.Session, error) {
	sessions := []*model.Session{}

	query := me.sessionSelectQuery.
		Where(sq.Eq{"UserId": userId}).
		OrderBy("LastActivityAt DESC")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "session_get_sessions_tosql")
	}

	err = me.GetReplica().Select(&sessions, sql, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Sessions with userId=%s", userId)
	}

	teamMembers, err := me.Team().GetTeamsForUser(rctx, userId, "", true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find TeamMembers for Session with userId=%s", userId)
	}

	for _, session := range sessions {
		session.TeamMembers = make([]*model.TeamMember, 0, len(teamMembers))
		for _, tm := range teamMembers {
			if tm.DeleteAt == 0 {
				session.TeamMembers = append(session.TeamMembers, tm)
			}
		}
	}
	return sessions, nil
}

// GetLRUSessions gets the Least Recently Used sessions from the store. Note: the use of limit and offset
// are intentional; they are hardcoded from the app layer (i.e., will not result in a non-performant query).
func (me SqlSessionStore) GetLRUSessions(rctx request.CTX, userId string, limit uint64, offset uint64) ([]*model.Session, error) {
	builder := me.sessionSelectQuery.
		Where(sq.Eq{"UserId": userId}).
		OrderBy("LastActivityAt DESC").
		Limit(limit).
		Offset(offset)
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_lru_sessions_tosql")
	}

	var sessions []*model.Session
	if err := me.GetReplica().Select(&sessions, query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find Sessions with userId=%s", userId)
	}
	return sessions, nil
}

func (me SqlSessionStore) GetSessionsWithActiveDeviceIds(userId string) ([]*model.Session, error) {
	now := model.GetMillis()

	// Start with the base query
	builder := me.sessionSelectQuery.
		Where(sq.Eq{"UserId": userId}).
		Where(sq.NotEq{"ExpiresAt": 0}).
		Where(sq.GtOrEq{"ExpiresAt": now}).
		Where(sq.NotEq{"DeviceId": ""})

	// Add the last_removed_device_id condition
	builder = builder.Where("DeviceId != COALESCE(Props->>'last_removed_device_id', '')")

	sessions := []*model.Session{}

	if err := me.GetReplica().SelectBuilder(&sessions, builder); err != nil {
		return nil, errors.Wrapf(err, "failed to find Sessions with userId=%s", userId)
	}
	return sessions, nil
}

func (me SqlSessionStore) GetMobileSessionMetadata() ([]*model.MobileSessionMetadata, error) {
	versionProp := model.SessionPropMobileVersion
	notificationDisabledProp := model.SessionPropDeviceNotificationDisabled
	platformQuery := "NULLIF(SPLIT_PART(deviceid, ':', 1), '')"

	query, args, err := me.getQueryBuilder().
		Select(fmt.Sprintf(
			"COUNT(userid) AS Count, COALESCE(%s,'N/A') AS Platform, COALESCE(props->>'%s','N/A') AS Version, COALESCE(props->>'%s','false') as NotificationDisabled",
			platformQuery,
			versionProp,
			notificationDisabledProp,
		)).
		From("Sessions").
		GroupBy("Platform", "Version", "NotificationDisabled").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "sessions_tosql")
	}

	versions := []*model.MobileSessionMetadata{}
	err = me.GetReplica().Select(&versions, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed get mobile session metadata")
	}
	return versions, nil
}

func (me SqlSessionStore) GetSessionsExpired(thresholdMillis int64, mobileOnly bool, unnotifiedOnly bool) ([]*model.Session, error) {
	now := model.GetMillis()
	builder := me.sessionSelectQuery.
		Where(sq.NotEq{"ExpiresAt": 0}).
		Where(sq.Lt{"ExpiresAt": now}).
		Where(sq.Gt{"ExpiresAt": now - thresholdMillis})
	if mobileOnly {
		builder = builder.Where(sq.NotEq{"DeviceId": ""})
	}
	if unnotifiedOnly {
		builder = builder.Where(sq.NotEq{"ExpiredNotify": true})
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "sessions_tosql")
	}

	sessions := []*model.Session{}

	err = me.GetReplica().Select(&sessions, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Sessions")
	}
	return sessions, nil
}

func (me SqlSessionStore) UpdateExpiredNotify(sessionId string, notified bool) error {
	query, args, err := me.getQueryBuilder().
		Update("Sessions").
		Set("ExpiredNotify", notified).
		Where(sq.Eq{"Id": sessionId}).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "sessions_tosql")
	}

	_, err = me.GetMaster().Exec(query, args...)
	if err != nil {
		return errors.Wrapf(err, "failed to update Session with id=%s", sessionId)
	}
	return nil
}

func (me SqlSessionStore) Remove(sessionIdOrToken string) error {
	_, err := me.GetMaster().Exec("DELETE FROM Sessions WHERE Id = ? Or Token = ?", sessionIdOrToken, sessionIdOrToken)
	if err != nil {
		return errors.Wrapf(err, "failed to delete Session with sessionIdOrToken=%s", sessionIdOrToken)
	}
	return nil
}

func (me SqlSessionStore) RemoveAllSessions() error {
	_, err := me.GetMaster().Exec("DELETE FROM Sessions")
	if err != nil {
		return errors.Wrap(err, "failed to delete all Sessions")
	}
	return nil
}

func (me SqlSessionStore) PermanentDeleteSessionsByUser(userId string) error {
	_, err := me.GetMaster().Exec("DELETE FROM Sessions WHERE UserId = ?", userId)
	if err != nil {
		return errors.Wrapf(err, "failed to delete Session with userId=%s", userId)
	}

	return nil
}

func (me SqlSessionStore) UpdateExpiresAt(sessionId string, time int64) error {
	_, err := me.GetMaster().Exec("UPDATE Sessions SET ExpiresAt = ?, ExpiredNotify = false WHERE Id = ?", time, sessionId)
	if err != nil {
		return errors.Wrapf(err, "failed to update Session with sessionId=%s", sessionId)
	}
	return nil
}

func (me SqlSessionStore) UpdateLastActivityAt(sessionId string, time int64) error {
	_, err := me.GetMaster().Exec("UPDATE Sessions SET LastActivityAt = ? WHERE Id = ?", time, sessionId)
	if err != nil {
		return errors.Wrapf(err, "failed to update Session with id=%s", sessionId)
	}
	return nil
}

func (me SqlSessionStore) UpdateRoles(userId, roles string) (string, error) {
	if len(roles) > model.UserRolesMaxLength {
		return "", fmt.Errorf("given session roles length (%d) exceeds max storage limit (%d)", len(roles), model.UserRolesMaxLength)
	}

	_, err := me.GetMaster().Exec("UPDATE Sessions SET Roles = ? WHERE UserId = ?", roles, userId)
	if err != nil {
		return "", errors.Wrapf(err, "failed to update Session with userId=%s and roles=%s", userId, roles)
	}
	return userId, nil
}

func (me SqlSessionStore) UpdateDeviceId(id string, deviceId string, expiresAt int64) (string, error) {
	query := "UPDATE Sessions SET DeviceId = ?, ExpiresAt = ?, ExpiredNotify = false WHERE Id = ?"

	_, err := me.GetMaster().Exec(query, deviceId, expiresAt, id)
	if err != nil {
		return "", errors.Wrapf(err, "failed to update Session with id=%s", id)
	}
	return deviceId, nil
}

func (me SqlSessionStore) UpdateProps(session *model.Session) error {
	jsonProps, err := json.Marshal(session.Props)
	if err != nil {
		return errors.Wrap(err, "failed marshalling session props")
	}
	if me.IsBinaryParamEnabled() {
		jsonProps = AppendBinaryFlag(jsonProps)
	}
	query, args, err := me.getQueryBuilder().
		Update("Sessions").
		Set("Props", jsonProps).
		Where(sq.Eq{"Id": session.Id}).
		ToSql()
	if err != nil {
		errors.Wrap(err, "sessions_tosql")
	}
	_, err = me.GetMaster().Exec(query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to update Session")
	}
	return nil
}

func (me SqlSessionStore) AnalyticsSessionCount() (int64, error) {
	var count int64
	query :=
		`SELECT
			COUNT(*)
		FROM
			Sessions
		WHERE ExpiresAt > ?`
	if err := me.GetReplica().Get(&count, query, model.GetMillis()); err != nil {
		return int64(0), errors.Wrap(err, "failed to count Sessions")
	}
	return count, nil
}

func (me SqlSessionStore) Cleanup(expiryTime int64, batchSize int64) error {
	var query string
	if me.DriverName() == model.DatabaseDriverPostgres {
		query = "DELETE FROM Sessions WHERE Id IN (SELECT Id FROM Sessions WHERE ExpiresAt != 0 AND ? > ExpiresAt LIMIT ?)"
	} else {
		query = "DELETE FROM Sessions WHERE ExpiresAt != 0 AND ? > ExpiresAt LIMIT ?"
	}

	var rowsAffected int64 = 1

	for rowsAffected > 0 {
		sqlResult, err := me.GetMaster().Exec(query, expiryTime, batchSize)
		if err != nil {
			return errors.Wrap(err, "unable to delete sessions")
		}
		var rowErr error
		rowsAffected, rowErr = sqlResult.RowsAffected()
		if rowErr != nil {
			return errors.Wrap(err, "unable to delete sessions")
		}

		time.Sleep(sessionsCleanupDelay)
	}

	return nil
}

// Desktop login methods

// GetSessionByDeviceCode retrieves a session by device code
func (me SqlSessionStore) GetSessionByDeviceCode(rctx request.CTX, deviceCode string) (*model.Session, error) {
	sessions := []*model.Session{}

	// Query sessions by Props->>'device_code' (JSON field)
	// This works for PostgreSQL and MySQL 5.7+
	query := `
		SELECT Id, Token, CreateAt, ExpiresAt, LastActivityAt, UserId, DeviceId, Roles, IsOAuth, ExpiredNotify, Props
		FROM Sessions
		WHERE JSON_UNQUOTE(JSON_EXTRACT(Props, '$.device_code')) = ?
		LIMIT 1
	`

	err := me.DBXFromContext(rctx.Context()).Select(&sessions, query, deviceCode)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find Session with deviceCode=%s", deviceCode)
	}
	if len(sessions) == 0 {
		return nil, store.NewErrNotFound("Session", fmt.Sprintf("deviceCode=%s", deviceCode))
	}

	return sessions[0], nil
}

// GetPendingSessionCount returns the count of pending sessions
func (me SqlSessionStore) GetPendingSessionCount(rctx request.CTX) (int, error) {
	var count int
	query := `
		SELECT COUNT(*)
		FROM Sessions
		WHERE JSON_UNQUOTE(JSON_EXTRACT(Props, '$.session_state')) = 'pending_activation'
	`

	err := me.GetReplica().Get(&count, query)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count pending sessions")
	}

	return count, nil
}

// GetPendingSessions returns pending desktop login sessions
func (me SqlSessionStore) GetPendingSessions(rctx request.CTX, limit int) ([]*model.Session, error) {
	sessions := []*model.Session{}

	query := `
		SELECT Id, Token, CreateAt, ExpiresAt, LastActivityAt, UserId, DeviceId, Roles, IsOAuth, ExpiredNotify, Props
		FROM Sessions
		WHERE JSON_UNQUOTE(JSON_EXTRACT(Props, '$.session_state')) = 'pending_activation'
		ORDER BY CreateAt DESC
		LIMIT ?
	`

	err := me.GetReplica().Select(&sessions, query, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find pending sessions")
	}

	return sessions, nil
}

// UpdateDeviceCodeSession updates a session with user information after device code verification
func (me SqlSessionStore) UpdateDeviceCodeSession(rctx request.CTX, session *model.Session) (*model.Session, error) {
	if session == nil || session.Id == "" {
		return nil, errors.New("session or session ID is required")
	}

	jsonProps, err := json.Marshal(session.Props)
	if err != nil {
		return nil, errors.Wrap(err, "failed marshalling session props")
	}

	if me.IsBinaryParamEnabled() {
		jsonProps = AppendBinaryFlag(jsonProps)
	}

	query, args, err := me.getQueryBuilder().
		Update("Sessions").
		Set("UserId", session.UserId).
		Set("Roles", session.Roles).
		Set("ExpiresAt", session.ExpiresAt).
		Set("Props", jsonProps).
		Where(sq.Eq{"Id": session.Id}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "update_device_code_session_tosql")
	}

	if _, err = me.GetMaster().Exec(query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to update Session with id=%s", session.Id)
	}

	// Fetch team members for the activated session
	teamMembers, err := me.Team().GetTeamsForUser(rctx, session.UserId, "", true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find TeamMembers for Session with userId=%s", session.UserId)
	}

	session.TeamMembers = make([]*model.TeamMember, 0, len(teamMembers))
	for _, tm := range teamMembers {
		if tm.DeleteAt == 0 {
			session.TeamMembers = append(session.TeamMembers, tm)
		}
	}

	return session, nil
}

// CleanupExpiredPendingSessions removes expired pending sessions
func (me SqlSessionStore) CleanupExpiredPendingSessions(rctx request.CTX, currentTime int64) (int, error) {
	query := `
		DELETE FROM Sessions
		WHERE JSON_UNQUOTE(JSON_EXTRACT(Props, '$.session_state')) = 'pending_activation'
		AND ExpiresAt < ?
	`

	result, err := me.GetMaster().Exec(query, currentTime)
	if err != nil {
		return 0, errors.Wrap(err, "failed to cleanup expired pending sessions")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return int(rowsAffected), nil
}

// UpdateExpiresAt updates session expiry time (already exists above, but adding for interface consistency)
func (me SqlSessionStore) UpdateExpiresAt(rctx request.CTX, sessionId string, expiresAt int64) error {
	_, err := me.GetMaster().Exec("UPDATE Sessions SET ExpiresAt = ?, ExpiredNotify = false WHERE Id = ?", expiresAt, sessionId)
	if err != nil {
		return errors.Wrapf(err, "failed to update Session with sessionId=%s", sessionId)
	}
	return nil
}

// Device Code Metadata methods - SECURITY FIX
// These store device codes WITHOUT creating sessions to prevent session fixation

// StoreDeviceCodeMetadata stores device code metadata in a separate table-like structure
// We use the Sessions table Props field but with a special marker to distinguish from real sessions
func (me SqlSessionStore) StoreDeviceCodeMetadata(rctx request.CTX, metadata *model.DeviceCodeMetadata) error {
	// Create a minimal "session" record that's really just metadata storage
	// UserId is empty, and we mark it with a special device_code_metadata state
	props := map[string]string{
		model.SessionPropDeviceCode:     metadata.DeviceCode,
		model.SessionPropDeviceName:     metadata.DeviceName,
		model.SessionPropDevicePlatform: metadata.DevicePlatform,
		model.SessionPropAppVersion:     metadata.AppVersion,
		model.SessionPropLoginMethod:    metadata.LoginMethod,
		model.SessionPropCreatedAt:      fmt.Sprintf("%d", metadata.CreatedAt),
		model.SessionPropSessionState:   "device_code_metadata", // Special marker
	}

	jsonProps, err := json.Marshal(props)
	if err != nil {
		return errors.Wrap(err, "failed marshalling device code metadata")
	}

	if me.IsBinaryParamEnabled() {
		jsonProps = AppendBinaryFlag(jsonProps)
	}

	// Use device code as the ID, empty token, empty UserId
	query, args, err := me.getQueryBuilder().
		Insert("Sessions").
		Columns("Id", "Token", "CreateAt", "ExpiresAt", "LastActivityAt", "UserId", "DeviceId", "Roles", "IsOAuth", "ExpiredNotify", "Props").
		Values(metadata.DeviceCode, "", metadata.CreatedAt, metadata.ExpiresAt, metadata.CreatedAt, "", metadata.DeviceCode, "", false, false, jsonProps).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "device_code_metadata_tosql")
	}

	if _, err = me.GetMaster().Exec(query, args...); err != nil {
		return errors.Wrapf(err, "failed to store device code metadata with code=%s", metadata.DeviceCode)
	}

	return nil
}

// GetDeviceCodeMetadata retrieves device code metadata
func (me SqlSessionStore) GetDeviceCodeMetadata(rctx request.CTX, deviceCode string) (*model.DeviceCodeMetadata, error) {
	var sessions []*model.Session

	query := me.sessionSelectQuery.
		Where(sq.Eq{"Id": deviceCode}).
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "get_device_code_metadata_tosql")
	}

	err = me.DBXFromContext(rctx.Context()).Select(&sessions, sql, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find device code metadata with code=%s", deviceCode)
	}
	if len(sessions) == 0 {
		return nil, store.NewErrNotFound("DeviceCodeMetadata", fmt.Sprintf("deviceCode=%s", deviceCode))
	}

	session := sessions[0]
	
	// Verify it's actually metadata, not a real session
	if state, ok := session.Props[model.SessionPropSessionState]; !ok || state != "device_code_metadata" {
		return nil, errors.New("invalid device code metadata")
	}

	// Convert back to metadata
	createdAt := int64(0)
	if createdStr, ok := session.Props[model.SessionPropCreatedAt]; ok {
		fmt.Sscanf(createdStr, "%d", &createdAt)
	}

	metadata := &model.DeviceCodeMetadata{
		DeviceCode:     session.Props[model.SessionPropDeviceCode],
		DeviceName:     session.Props[model.SessionPropDeviceName],
		DevicePlatform: session.Props[model.SessionPropDevicePlatform],
		AppVersion:     session.Props[model.SessionPropAppVersion],
		LoginMethod:    session.Props[model.SessionPropLoginMethod],
		CreatedAt:      createdAt,
		ExpiresAt:      session.ExpiresAt,
	}

	return metadata, nil
}

// DeleteDeviceCodeMetadata deletes device code metadata
func (me SqlSessionStore) DeleteDeviceCodeMetadata(rctx request.CTX, deviceCode string) error {
	_, err := me.GetMaster().Exec("DELETE FROM Sessions WHERE Id = ?", deviceCode)
	if err != nil {
		return errors.Wrapf(err, "failed to delete device code metadata with code=%s", deviceCode)
	}
	return nil
}

// GetPendingDeviceCodeCount returns the count of pending device codes
func (me SqlSessionStore) GetPendingDeviceCodeCount(rctx request.CTX) (int, error) {
	var count int
	query := `
		SELECT COUNT(*)
		FROM Sessions
		WHERE JSON_UNQUOTE(JSON_EXTRACT(Props, '$.session_state')) = 'device_code_metadata'
	`

	err := me.GetReplica().Get(&count, query)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count device code metadata")
	}

	return count, nil
}

// CleanupExpiredDeviceCodes removes expired device code metadata
func (me SqlSessionStore) CleanupExpiredDeviceCodes(rctx request.CTX, currentTime int64) (int, error) {
	query := `
		DELETE FROM Sessions
		WHERE JSON_UNQUOTE(JSON_EXTRACT(Props, '$.session_state')) = 'device_code_metadata'
		AND ExpiresAt < ?
	`

	result, err := me.GetMaster().Exec(query, currentTime)
	if err != nil {
		return 0, errors.Wrap(err, "failed to cleanup expired device codes")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return int(rowsAffected), nil
}

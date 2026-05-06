package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type ApiCallLog struct {
	Id                 int    `json:"id" gorm:"index:idx_api_call_created_at_id,priority:1"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);index:idx_api_call_request_attempt,priority:1;index:idx_api_call_request_id;default:''"`
	RetryIndex         int    `json:"retry_index" gorm:"index:idx_api_call_request_attempt,priority:2;default:0"`
	UserId             int    `json:"user_id" gorm:"index"`
	Username           string `json:"username" gorm:"index;default:''"`
	TokenId            int    `json:"token_id" gorm:"index;default:0"`
	TokenName          string `json:"token_name" gorm:"index;default:''"`
	ChannelId          int    `json:"channel_id" gorm:"index"`
	ChannelType        int    `json:"channel_type" gorm:"default:0"`
	ModelName          string `json:"model_name" gorm:"index;default:''"`
	UpstreamModelName  string `json:"upstream_model_name" gorm:"index;default:''"`
	Group              string `json:"group" gorm:"index;default:''"`
	Method             string `json:"method" gorm:"type:varchar(16);default:''"`
	RequestPath        string `json:"request_path" gorm:"type:varchar(255);index;default:''"`
	RelayFormat        string `json:"relay_format" gorm:"type:varchar(32);default:''"`
	FinalRequestFormat string `json:"final_request_format" gorm:"type:varchar(32);default:''"`
	IsStream           bool   `json:"is_stream"`
	StatusCode         int    `json:"status_code" gorm:"default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);index;default:'started'"`
	Error              string `json:"error" gorm:"type:text"`
	RequestBody        string `json:"request_body" gorm:"type:text"`
	ResponseBody       string `json:"response_body" gorm:"type:text"`
	ResponseText       string `json:"response_text" gorm:"type:text"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;index:idx_api_call_created_at_id,priority:2"`
	CompletedAt        int64  `json:"completed_at" gorm:"bigint;index"`
	DurationMs         int64  `json:"duration_ms" gorm:"default:0"`
	Other              string `json:"other" gorm:"type:text"`
}

func (ApiCallLog) TableName() string {
	return "api_call_logs"
}

type RecordApiCallRequestParams struct {
	RequestId          string
	RetryIndex         int
	UserId             int
	Username           string
	TokenId            int
	TokenName          string
	ChannelId          int
	ChannelType        int
	ModelName          string
	UpstreamModelName  string
	Group              string
	Method             string
	RequestPath        string
	RelayFormat        string
	FinalRequestFormat string
	IsStream           bool
	RequestBody        string
	Other              map[string]interface{}
}

type UpdateApiCallResponseParams struct {
	RequestId          string
	RetryIndex         int
	ChannelId          int
	StatusCode         int
	Status             string
	Error              string
	ResponseBody       string
	ResponseText       string
	CompletedAt        int64
	DurationMs         int64
	FinalRequestFormat string
	Other              map[string]interface{}
}

func apiCallLogEnabled() bool {
	return common.ApiCallLogEnabled && LOG_DB != nil
}

func RecordApiCallRequest(params RecordApiCallRequestParams) {
	if !apiCallLogEnabled() || strings.TrimSpace(params.RequestId) == "" {
		return
	}
	now := common.GetTimestamp()
	log := &ApiCallLog{
		RequestId:          params.RequestId,
		RetryIndex:         params.RetryIndex,
		UserId:             params.UserId,
		Username:           params.Username,
		TokenId:            params.TokenId,
		TokenName:          params.TokenName,
		ChannelId:          params.ChannelId,
		ChannelType:        params.ChannelType,
		ModelName:          params.ModelName,
		UpstreamModelName:  params.UpstreamModelName,
		Group:              params.Group,
		Method:             params.Method,
		RequestPath:        params.RequestPath,
		RelayFormat:        params.RelayFormat,
		FinalRequestFormat: params.FinalRequestFormat,
		IsStream:           params.IsStream,
		Status:             "started",
		RequestBody:        params.RequestBody,
		CreatedAt:          now,
		Other:              common.MapToJsonStr(params.Other),
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record api call request: " + err.Error())
	}
}

func UpdateApiCallResponse(params UpdateApiCallResponseParams) {
	if !apiCallLogEnabled() || strings.TrimSpace(params.RequestId) == "" {
		return
	}
	completedAt := params.CompletedAt
	if completedAt == 0 {
		completedAt = common.GetTimestamp()
	}
	status := params.Status
	if status == "" {
		if params.Error != "" || params.StatusCode >= 400 {
			status = "error"
		} else {
			status = "completed"
		}
	}
	updates := map[string]interface{}{
		"status_code":  params.StatusCode,
		"status":       status,
		"error":        params.Error,
		"completed_at": completedAt,
		"duration_ms":  params.DurationMs,
	}
	if params.ResponseBody != "" {
		updates["response_body"] = params.ResponseBody
	}
	if params.ResponseText != "" {
		updates["response_text"] = params.ResponseText
	}
	if params.FinalRequestFormat != "" {
		updates["final_request_format"] = params.FinalRequestFormat
	}
	if params.Other != nil {
		updates["other"] = common.MapToJsonStr(params.Other)
	}

	tx := LOG_DB.Model(&ApiCallLog{}).
		Where("request_id = ? AND retry_index = ? AND channel_id = ?", params.RequestId, params.RetryIndex, params.ChannelId).
		Updates(updates)
	if tx.Error != nil {
		common.SysLog("failed to update api call response: " + tx.Error.Error())
		return
	}
	if tx.RowsAffected > 0 {
		return
	}

	log := &ApiCallLog{
		RequestId:          params.RequestId,
		RetryIndex:         params.RetryIndex,
		ChannelId:          params.ChannelId,
		StatusCode:         params.StatusCode,
		Status:             status,
		Error:              params.Error,
		ResponseBody:       params.ResponseBody,
		ResponseText:       params.ResponseText,
		CreatedAt:          completedAt,
		CompletedAt:        completedAt,
		DurationMs:         params.DurationMs,
		FinalRequestFormat: params.FinalRequestFormat,
		Other:              common.MapToJsonStr(params.Other),
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog(fmt.Sprintf("failed to create fallback api call response log: %s", err.Error()))
	}
}

func ApiCallDurationMs(start time.Time) int64 {
	if start.IsZero() {
		return 0
	}
	return time.Since(start).Milliseconds()
}

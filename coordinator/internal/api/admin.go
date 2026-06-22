package api

import (
	"net/http"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

type createAppReq struct {
	Name       string          `json:"name"`
	WebhookURL string          `json:"webhook_url"`
	Config     store.AppConfig `json:"config"`
}

type createAppResp struct {
	AppID         string `json:"app_id"`
	APIKey        string `json:"api_key"` // shown once
	APIKeyPrefix  string `json:"api_key_prefix"`
	WebhookSecret string `json:"webhook_secret"`
}

func (s *Server) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	var req createAppReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}

	key, hash, prefix, err := auth.GenerateKey("ctv")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "key generation failed")
		return
	}
	webhookSecret, err := auth.GenerateSecret()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "secret generation failed")
		return
	}

	app, err := s.store.CreateApp(r.Context(), store.App{
		Name:          req.Name,
		APIKeyHash:    hash,
		APIKeyPrefix:  prefix,
		WebhookURL:    req.WebhookURL,
		WebhookSecret: webhookSecret,
		Config:        req.Config,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not create app")
		return
	}

	writeJSON(w, http.StatusCreated, createAppResp{
		AppID:         app.ID,
		APIKey:        key,
		APIKeyPrefix:  prefix,
		WebhookSecret: webhookSecret,
	})
}

type createDeviceReq struct {
	AppID        string   `json:"app_id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Capabilities []string `json:"capabilities"`
}

type createDeviceResp struct {
	DeviceID     string `json:"device_id"`
	DeviceSecret string `json:"device_secret"` // shown once
}

func (s *Server) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var req createDeviceReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AppID == "" || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "app_id and name are required")
		return
	}
	if req.Type != "android" && req.Type != "pi" {
		writeErr(w, http.StatusBadRequest, "bad_request", "type must be android or pi")
		return
	}

	secret, err := auth.GenerateSecret()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "secret generation failed")
		return
	}

	device, err := s.store.CreateDevice(r.Context(), store.Device{
		AppID:        req.AppID,
		Name:         req.Name,
		DeviceSecret: secret,
		Type:         req.Type,
		Capabilities: req.Capabilities,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not create device")
		return
	}

	writeJSON(w, http.StatusCreated, createDeviceResp{DeviceID: device.ID, DeviceSecret: secret})
}

type createNumberReq struct {
	DeviceID string   `json:"device_id"`
	MSISDN   string   `json:"msisdn"`
	Channels []string `json:"channels"`
}

func (s *Server) handleCreateNumber(w http.ResponseWriter, r *http.Request) {
	var req createNumberReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.DeviceID == "" || req.MSISDN == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "device_id and msisdn are required")
		return
	}

	number, err := s.store.CreateNumber(r.Context(), store.Number{
		DeviceID: req.DeviceID,
		MSISDN:   req.MSISDN,
		Channels: req.Channels,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not create number")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"number_id": number.ID})
}

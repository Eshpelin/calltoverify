package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Eshpelin/calltoverify/coordinator/internal/verify"
)

type numberView struct {
	MSISDN   string   `json:"msisdn"`
	Channels []string `json:"channels"`
	Status   string   `json:"status"`
}

func (s *Server) deviceNumbers(r *http.Request, deviceID string) ([]numberView, error) {
	nums, err := s.store.ListNumbersByDevice(r.Context(), deviceID)
	if err != nil {
		return nil, err
	}
	views := make([]numberView, 0, len(nums))
	for _, n := range nums {
		views = append(views, numberView{MSISDN: n.MSISDN, Channels: n.Channels, Status: n.Status})
	}
	return views, nil
}

// handleDeviceRegister returns the device's current config and number list, and
// marks it online.
func (s *Server) handleDeviceRegister(w http.ResponseWriter, r *http.Request) {
	device := deviceFromCtx(r)
	if err := s.store.SetHeartbeat(r.Context(), device.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "register failed")
		return
	}
	numbers, err := s.deviceNumbers(r, device.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not list numbers")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":    device.ID,
		"type":         device.Type,
		"capabilities": device.Capabilities,
		"numbers":      numbers,
	})
}

// handleDeviceHeartbeat refreshes liveness and returns the device's numbers.
func (s *Server) handleDeviceHeartbeat(w http.ResponseWriter, r *http.Request) {
	device := deviceFromCtx(r)
	if err := s.store.SetHeartbeat(r.Context(), device.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "heartbeat failed")
		return
	}
	numbers, err := s.deviceNumbers(r, device.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not list numbers")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "numbers": numbers})
}

type inboundReq struct {
	Number string `json:"number"`
	Type   string `json:"type"`
	Sender string `json:"sender"`
	Body   string `json:"body"`
}

func (s *Server) handleInbound(w http.ResponseWriter, r *http.Request) {
	device := deviceFromCtx(r)

	var req inboundReq
	if body := bodyFromCtx(r); len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}
	}
	if req.Number == "" || req.Sender == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "number and sender are required")
		return
	}

	res, err := s.svc.Inbound(r.Context(), device, verify.InboundRequest{
		Number: req.Number,
		Type:   req.Type,
		Sender: req.Sender,
		Body:   req.Body,
	})
	if err != nil {
		var ve *verify.ValidationError
		switch {
		case errors.As(err, &ve):
			writeErr(w, http.StatusBadRequest, "bad_request", ve.Error())
		case errors.Is(err, verify.ErrForbidden):
			writeErr(w, http.StatusForbidden, "forbidden", "device does not own this number")
		default:
			s.logger.Error("inbound", "err", err)
			writeErr(w, http.StatusInternalServerError, "internal", "inbound processing failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, res)
}

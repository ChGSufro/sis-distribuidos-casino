package handler

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Status  int    `json:"status"`
}

func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: statusCode >= 200 && statusCode < 300,
		Data:    data,
		Status:  statusCode,
	})
}

func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   message,
		Status:  statusCode,
	})
}

func WriteGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpStatus := http.StatusInternalServerError
	switch st.Code() {
	case codes.InvalidArgument:
		httpStatus = http.StatusBadRequest
	case codes.NotFound:
		httpStatus = http.StatusNotFound
	case codes.AlreadyExists:
		httpStatus = http.StatusConflict
	case codes.PermissionDenied:
		httpStatus = http.StatusForbidden
	case codes.Unauthenticated:
		httpStatus = http.StatusUnauthorized
	case codes.FailedPrecondition:
		httpStatus = http.StatusBadRequest
	case codes.ResourceExhausted:
		httpStatus = http.StatusTooManyRequests
	case codes.DeadlineExceeded:
		httpStatus = http.StatusGatewayTimeout
	case codes.Unavailable:
		httpStatus = http.StatusServiceUnavailable
	}

	WriteError(w, httpStatus, st.Message())
}

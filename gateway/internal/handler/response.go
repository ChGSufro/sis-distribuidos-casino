package handler

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Status  int    `json:"status"`
}

// WriteJSON responde con data envuelta en APIResponse. Los mensajes protobuf
// se serializan con protojson (no encoding/json) porque los tags generados
// incluyen omitempty, que ELIMINA campos con valor 0 (p. ej. seat_position: 0
// del asiento 0, current_turn, dealer_position) y rompe a los clientes.
// EmitUnpopulated garantiza que todos los campos viajen siempre.
func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	payload := any(data)
	if msg, ok := data.(proto.Message); ok {
		raw, err := protojson.MarshalOptions{
			UseProtoNames:   true, // mantiene snake_case (contrato del frontend)
			EmitUnpopulated: true, // envia tambien los campos en 0 o vacios
		}.Marshal(msg)
		if err == nil {
			payload = json.RawMessage(raw)
		}
	}

	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: statusCode >= 200 && statusCode < 300,
		Data:    payload,
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

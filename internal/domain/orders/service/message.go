package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

func OrderExpirationIDFromMessage(message redis.XMessage) (int, error) {
	if value, ok := message.Values["order_id"]; ok {
		return strconv.Atoi(toString(value))
	}
	if value, ok := message.Values["payload"]; ok {
		var payload struct {
			OrderID int `json:"order_id"`
		}
		if err := json.Unmarshal([]byte(toString(value)), &payload); err != nil {
			return 0, err
		}
		if payload.OrderID > 0 {
			return payload.OrderID, nil
		}
	}
	return 0, errors.New("order_id is required")
}

func toString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

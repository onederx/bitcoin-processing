package testutil

import (
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/settings"
)

type SettingsMock struct {
	settings.Settings

	Data map[string]interface{}
}

func (s *SettingsMock) GetString(key string) string {
	val, ok := s.Data[key]

	if !ok {
		return ""
	}
	st, ok := val.(string)

	if !ok {
		return ""
	}
	return st
}

func (s *SettingsMock) GetStringMandatory(key string) string {
	val, ok := s.Data[key]

	if !ok {
		return ""
	}
	st, ok := val.(string)

	if !ok {
		return ""
	}
	return st
}

func (s *SettingsMock) GetInt(key string) int {
	val, ok := s.Data[key]

	if !ok {
		return 0
	}
	i, ok := val.(int)

	if !ok {
		return 0
	}
	return i
}

func (s *SettingsMock) GetBTCAmount(key string) bitcoin.BTCAmount {
	val, ok := s.Data[key]

	if !ok {
		return bitcoin.BTCAmount(0)
	}

	a, ok := val.(bitcoin.BTCAmount)

	if !ok {
		return bitcoin.BTCAmount(0)
	}
	return a
}

package model

import "fmt"

type Config struct {
	ID    uint         `gorm:"column:id;primaryKey"`
	Email EmailSetting `gorm:"column:email_setting;serializer:json"`
	Mqtt  MqttSetting  `gorm:"column:mqtt_setting;serializer:json"`
}

type MqttSetting struct {
	Address    string `json:"address"`
	Topic      string `json:"topic"`
	ClientID   string `json:"clientId"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	CertFile   string `json:"certfile"`
	KeyFile    string `json:"keyfile"`
	SkipVerify bool   `json:"skipverify"`
}
type EmailSetting struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Authcode string   `json:"authcode"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

func (cfg Config) ValidateConfig() []error {
	var errs []error
	errs = append(errs, cfg.Mqtt.validate()...)
	errs = append(errs, cfg.Email.validate()...)
	return errs
}

// validate 检查 mqtt 配置：仅当该渠道被配置（任一字段非空）时才校验必填项，
// 未配置则视为不使用该渠道，跳过。username/password 可选（允许匿名 broker），
// certfile 可选（SkipVerify 时不需要）。
func (m MqttSetting) validate() []error {
	configured := m.Address != "" || m.Topic != "" || m.Username != "" ||
		m.Password != "" || m.CertFile != "" || m.SkipVerify
	if !configured {
		return nil
	}
	var errs []error
	required := []struct {
		name string
		ok   bool
	}{
		{"broker", m.Address != ""},
		{"topic", m.Topic != ""},
	}
	for _, r := range required {
		if !r.ok {
			errs = append(errs, fmt.Errorf("mqtt: %s 不能为空", r.name))
		}
	}
	return errs
}

// validate 检查 email 配置：同样仅在被配置时校验。email 各字段都是发信必需的。
func (e EmailSetting) validate() []error {
	configured := e.Host != "" || e.Username != "" || e.Authcode != "" ||
		e.From != "" || len(e.To) > 0 || e.Port != 0
	if !configured {
		return nil
	}
	var errs []error
	required := []struct {
		name string
		ok   bool
	}{
		{"host", e.Host != ""},
		{"port", e.Port > 0},
		{"username", e.Username != ""},
		{"authcode", e.Authcode != ""},
		{"from", e.From != ""},
		{"to", len(e.To) > 0},
	}
	for _, r := range required {
		if !r.ok {
			errs = append(errs, fmt.Errorf("email: %s 不能为空", r.name))
		}
	}
	return errs
}
